package testing

import (
	"context"
	"encoding/binary"
	"errors"
	"time"

	"github.com/dusk-network/dusk-blockchain/pkg/core/consensus"
	"github.com/dusk-network/dusk-blockchain/pkg/core/data/block"
	"github.com/dusk-network/dusk-blockchain/pkg/core/database"
	"github.com/dusk-network/dusk-blockchain/pkg/core/verifiers"
	"github.com/dusk-network/dusk-blockchain/pkg/p2p/wire/message"
	"github.com/dusk-network/dusk-blockchain/pkg/p2p/wire/topics"
	"github.com/dusk-network/dusk-blockchain/pkg/util/nativeutils/eventbus"
	"github.com/dusk-network/dusk-blockchain/pkg/util/nativeutils/rpcbus"
	crypto "github.com/dusk-network/dusk-crypto/hash"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// mockAcceptor accepts block on topics.Certificate, topics.Block and provides VerifyCandidateBlock
// mockAcceptor owns mockConsensusRegistry and has direct read/write access to it
//nolint:unused
type mockAcceptor struct {
	certficateChan           chan message.Message
	blockChan                chan message.Message
	verifyCandidateBlockChan chan rpcbus.Request

	// reference to chain state (not owner)
	db  database.DB
	reg *mockSafeRegistry

	// TODO mock executor
}

//nolint:unused
func newMockAcceptor(e consensus.Emitter, db database.DB, reg *mockSafeRegistry) (*mockAcceptor, error) {

	// Subscriptions

	certficateChan := make(chan message.Message, 1)
	chanListener := eventbus.NewChanListener(certficateChan)
	e.EventBus.Subscribe(topics.Certificate, chanListener)

	blockChan := make(chan message.Message, 1)
	chanListener = eventbus.NewChanListener(blockChan)
	e.EventBus.Subscribe(topics.Block, chanListener)

	verifyCandidateBlockChan := make(chan rpcbus.Request, 1)
	if err := e.RPCBus.Register(topics.VerifyCandidateBlock, verifyCandidateBlockChan); err != nil {
		return nil, err
	}

	acc := mockAcceptor{
		certficateChan:           certficateChan,
		blockChan:                blockChan,
		verifyCandidateBlockChan: verifyCandidateBlockChan,
		db:                       db,
		reg:                      reg,
	}

	// Initialize always genesis with a current chain tip
	// Suitable only in testing
	chainTip := reg.GetChainTip()
	err := db.Update(func(t database.Transaction) error {
		return t.StoreBlock(&chainTip)
	})

	if err != nil {
		return nil, err
	}

	// TODO: Should this stay here?
	err = db.Update(func(t database.Transaction) error {
		k, _ := crypto.RandEntropy(32)
		d, _ := crypto.RandEntropy(32)
		indexStoredBidBytes, _ := crypto.RandEntropy(8)
		return t.StoreBidValues(d, k, binary.LittleEndian.Uint64(indexStoredBidBytes), 250000)
	})

	return &acc, err
}

// acceptBlock verifies block from consensus perspective
// Non-duplicated block
// Valid block certificate
// Valid block header
func (a *mockAcceptor) acceptBlock(b block.Block) error {

	// 1. Check the certificate
	if err := sanityCheckBlock(a.db, a.reg.GetChainTip(), b); err != nil {
		return err
	}

	// 2. Check the certificate
	// This check should avoid a possible race condition between accepting two blocks
	// at the same height, as the probability of the committee creating two valid certificates
	// for the same round is negligible.
	if err := verifiers.CheckBlockCertificate(a.reg.GetProvisioners(), b); err != nil {
		return err
	}

	// Store block in the in-memory database
	err := a.db.Update(func(t database.Transaction) error {
		return t.StoreBlock(&b)
	})

	if err != nil {
		return err
	}

	// Update registry
	a.reg.SetChainTip(b)

	// Gossip/Kadcast here topics.Block
	// Not needed for the purposes of consensus sandbox

	return err
}

func (a *mockAcceptor) processCandidateVerificationRequest(r rpcbus.Request) {
	var res rpcbus.Response

	cm := r.Params.(message.Candidate)
	candidateBlock := *cm.Block
	chainTip := a.reg.GetChainTip()

	// We first perform a quick check on the Block Header and
	if err := sanityCheckBlock(a.db, chainTip, candidateBlock); err != nil {
		res.Err = err
		r.RespChan <- res
		return
	}

	/* VST
	_, err := c.executor.VerifyStateTransition(c.ctx, candidateBlock.Txs, candidateBlock.Header.Height)
	if err != nil {
		res.Err = err
		r.RespChan <- res
		return
	}
	*/

	r.RespChan <- res
}

func (a *mockAcceptor) loop(pctx context.Context, restartLoopChan chan bool, assert *assert.Assertions) {

	for {
		select {
		// Handles Idle
		case <-time.After(30 * time.Second):
			logrus.Warn("acceptor on idle")
		// Handles topics.Certificate from consensus
		case m := <-a.certficateChan:

			// Extract winning hash and block certificate
			cMsg := m.Payload().(message.Certificate)
			certificate := cMsg.Ag.GenerateCertificate()
			winningHash := cMsg.Ag.State().BlockHash

			// Try to fetch block by hash from local registry
			cm, err := a.reg.GetCandidateByHash(winningHash)
			assert.NoError(err)
			// TODO: if err, FetchCandidate from Network (Peers in Gossip)

			cm.Block.Header.Certificate = certificate
			// Ensure block is accepted by Chain
			err = a.acceptBlock(*cm.Block)
			assert.NoError(err)

			restartLoopChan <- true

		// Handles topics.Block from the wire on synchronizing
		case _ = <-a.blockChan:
			// Not needed in testing for now
			//err = a.acceptBlock(*cm.Block)
			//assert.NoError(err)
		case req := <-a.verifyCandidateBlockChan:
			a.processCandidateVerificationRequest(req)
		case <-pctx.Done():
			return
		}
	}
}

func sanityCheckBlock(db database.DB, prevBlock block.Block, b block.Block) error {

	// 1. Check if the block is a duplicate
	err := db.View(func(t database.Transaction) error {
		_, err := t.FetchBlockExists(b.Header.Hash)
		return err
	})

	if err != database.ErrBlockNotFound {
		if err == nil {
			err = errors.New("block already exists")
		}
		return err
	}

	// SanityCheck block header to ensure consensus has worked a chainTip
	if err = verifiers.CheckBlockHeader(prevBlock, b); err != nil {
		return err
	}

	if err = verifiers.CheckMultiCoinbases(b.Txs); err != nil {
		return err
	}

	return nil
}
