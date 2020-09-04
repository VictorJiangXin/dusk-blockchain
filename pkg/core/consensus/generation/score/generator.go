package score

import (
	"context"
	"errors"
	"sync"

	"github.com/dusk-network/dusk-blockchain/pkg/core/consensus"
	"github.com/dusk-network/dusk-blockchain/pkg/core/consensus/header"
	"github.com/dusk-network/dusk-blockchain/pkg/core/consensus/key"
	"github.com/dusk-network/dusk-blockchain/pkg/core/data/ipc/blindbid"
	"github.com/dusk-network/dusk-blockchain/pkg/core/data/ipc/common"
	"github.com/dusk-network/dusk-blockchain/pkg/core/data/ipc/transactions"
	"github.com/dusk-network/dusk-blockchain/pkg/p2p/wire/message"
	"github.com/dusk-network/dusk-blockchain/pkg/p2p/wire/topics"
	"github.com/dusk-network/dusk-blockchain/pkg/util/nativeutils/eventbus"
	"github.com/dusk-network/dusk-crypto/bls"
	log "github.com/sirupsen/logrus"
)

var _ consensus.Component = (*Generator)(nil)

var emptyHash [32]byte
var lg = log.WithField("process", "score generator")

// Generator is responsible for generating the proof of blind bid, along with the score.
// It forwards the resulting information to the candidate generator, in a ScoreEvent
// message.
type Generator struct {
	publisher eventbus.Publisher
	roundInfo consensus.RoundUpdate
	k         *common.BlsScalar
	key.Keys
	seed      *common.BlsScalar
	secret    *common.JubJubCompressed
	lock      sync.RWMutex
	threshold *consensus.Threshold
	index     uint64

	signer         consensus.Signer
	generationID   uint32
	generationName string
	bg             transactions.BlockGenerator
	ctx            context.Context
}

// NewComponent returns an uninitialized Generator.
func NewComponent(ctx context.Context, publisher eventbus.Publisher, k, seed *common.BlsScalar, secret *common.JubJubCompressed, bg transactions.BlockGenerator, consensusKeys key.Keys) *Generator {
	return &Generator{
		publisher: publisher,
		k:         k,
		Keys:      consensusKeys,
		seed:      seed,
		secret:    secret,
		threshold: consensus.NewThreshold(),
		bg:        bg,
		ctx:       ctx,
	}
}

// scoreFactory is used to compose the Score message before propagating it
// internally. It is meant to be instantiated every time a new score needs to
// be created
type scoreFactory struct {
	seed  []byte
	score blindbid.GenerateScoreResponse
}

func newFactory(seed []byte, score blindbid.GenerateScoreResponse) scoreFactory {
	return scoreFactory{seed, score}
}

// Create complies with the consensus.PacketFactory interface
func (sf scoreFactory) Create(sender []byte, round uint64, step uint8) consensus.InternalPacket {
	hdr := header.Header{
		Round:     round,
		Step:      step,
		BlockHash: emptyHash[:],
		PubKeyBLS: sender,
	}

	return message.NewScoreProposal(hdr, sf.seed, sf.score)
}

// Initialize the Generator, by creating the round seed and returning the Listener
// for the Generation topic, which triggers the Generator.
// Implements consensus.Component.
func (g *Generator) Initialize(eventPlayer consensus.EventPlayer, signer consensus.Signer, ru consensus.RoundUpdate) []consensus.TopicListener {
	g.signer = signer
	g.roundInfo = ru
	signedSeed, err := g.sign(ru.Seed)
	if err != nil {
		lg.WithField("category", "BUG").WithError(err).Errorln("could not sign seed")
		return nil
	}

	g.seed = &common.BlsScalar{Data: signedSeed}

	// TODO: RUSK is the only one to know if this generator is in the bid list.
	// If we are not, we should probably not receive messages (akin to the old
	// and now defunct inBidList method). This can be implemented later though
	generationSubscriber := consensus.TopicListener{
		Topic:    topics.Generation,
		Listener: consensus.NewSimpleListener(g.Collect, consensus.LowPriority, false),
	}
	g.generationID = generationSubscriber.Listener.ID()
	g.generationName = "score/Generator"

	return []consensus.TopicListener{generationSubscriber}
}

// ID returns the listener ID of the Generator.
// Implements consensus.Component.
func (g *Generator) ID() uint32 {
	return g.generationID
}

// Name returns the listener Name of the Generator.
// Implements consensus.Component.
func (g *Generator) Name() string {
	return g.generationName
}

// Finalize implements consensus.Component.
func (g *Generator) Finalize() {}

// Collect complies to the consensus.Component interface
func (g *Generator) Collect(e consensus.InternalPacket) error {
	defer func() {
		g.lock.Lock()
		defer g.lock.Unlock()
		g.threshold.Lower()
	}()
	return g.generateScore(e.State().Step)
}

func (g *Generator) generateScore(step uint8) error {
	sr := blindbid.GenerateScoreRequest{
		K:              g.k,
		Seed:           g.seed,
		Secret:         g.secret,
		Round:          uint32(g.roundInfo.Round),
		Step:           uint32(step),
		IndexStoredBid: g.index,
	}
	scoreTx, err := g.bg.GenerateScore(g.ctx, sr)
	// GenerateScore would return error if we are not in this round bidlist, or
	// if the BidTransaction expired or is malformed
	// TODO: check the error and, if we are not in the bidlist, finalize the
	// component
	if err != nil {
		return err
	}

	g.lock.RLock()
	defer g.lock.RUnlock()
	if g.threshold.Exceeds(scoreTx.Score.Data) {
		return errors.New("proof score is below threshold")
	}

	score := g.signer.Compose(newFactory(g.seed.Data, scoreTx))
	msg := message.New(topics.ScoreEvent, score)
	return g.signer.SendInternally(topics.ScoreEvent, msg, g.ID())
}

func (g *Generator) sign(seed []byte) ([]byte, error) {
	signedSeed, err := bls.Sign(g.Keys.BLSSecretKey, g.Keys.BLSPubKey, seed)
	if err != nil {
		return nil, err
	}
	compSeed := signedSeed.Compress()
	return compSeed, nil
}
