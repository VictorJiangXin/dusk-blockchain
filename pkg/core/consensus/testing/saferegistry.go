package testing

import (
	"bytes"
	"errors"
	"sync"

	"github.com/dusk-network/dusk-blockchain/pkg/core/consensus/user"
	"github.com/dusk-network/dusk-blockchain/pkg/core/data/block"
	"github.com/dusk-network/dusk-blockchain/pkg/p2p/wire/message"
)

// ConsensusRegistry holds all consensus-related data structures
// It should provide concurrency-safe accessors
//nolint:unused
type mockSafeRegistry struct {

	// lock protection per registry instance
	// Option: a mutex instance per a member
	lock sync.RWMutex

	p               user.Provisioners
	lastCertificate *block.Certificate
	lastCommittee   [][]byte
	chainTip        block.Block
	candidates      []message.Candidate
}

//nolint:unused
func newMockSafeRegistry(genesis block.Block, cert *block.Certificate) *mockSafeRegistry {

	return &mockSafeRegistry{
		chainTip:        genesis,
		lastCertificate: cert,
	}
}

// RetrieveCandidate returns a copy of candidate block if found by hash
func (r *mockSafeRegistry) GetCandidateByHash(hash []byte) (message.Candidate, error) {

	r.lock.RLock()
	defer r.lock.RUnlock()

	if len(hash) != 32 {
		return message.Candidate{}, errors.New("invalid hash")
	}

	for n := 0; n < len(r.candidates); n++ {
		cm := r.candidates[n]
		b := cm.Block
		if bytes.Equal(b.Header.Hash, hash) {
			return cm.Copy().(message.Candidate), nil
		}
	}

	return message.Candidate{}, errors.New("candidate not found")
}

func (r *mockSafeRegistry) GetProvisioners() user.Provisioners {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.p.Copy()
}

func (r *mockSafeRegistry) GetChainTip() block.Block {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.chainTip.Copy().(block.Block)
}

func (r *mockSafeRegistry) GetLastCertificate() *block.Certificate {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.lastCertificate.Copy()
}

func (r *mockSafeRegistry) GetLastCommittee() [][]byte {
	r.lock.RLock()
	defer r.lock.RUnlock()

	dup := make([][]byte, len(r.lastCommittee))
	for i := range r.lastCommittee {
		dup[i] = make([]byte, len(r.lastCommittee[i]))
		copy(dup[i], r.lastCommittee[i])
	}

	return dup
}

func (r *mockSafeRegistry) SetProvisioners(p user.Provisioners) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.p = p.Copy()
}

func (r *mockSafeRegistry) SetChainTip(b block.Block) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.chainTip = b.Copy().(block.Block)
}

func (r *mockSafeRegistry) AddCandidate(m message.Candidate) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if len(r.candidates) == 0 {
		r.candidates = make([]message.Candidate, 0)
	}
	r.candidates = append(r.candidates, m)
}

// ResetCandidates cleans up all obsolete candidates
func (r *mockSafeRegistry) ResetCandidates(round uint64) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	candidates := make([]message.Candidate, 0)
	for n := 0; n < len(r.candidates); n++ {
		cm := r.candidates[n]
		b := cm.Block
		if b.Header.Height > round {
			candidates = append(candidates, cm)
		}
	}

	r.candidates = candidates
}
