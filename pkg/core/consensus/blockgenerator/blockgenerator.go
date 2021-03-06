package blockgenerator

import (
	"github.com/dusk-network/dusk-blockchain/pkg/core/consensus"
	"github.com/dusk-network/dusk-blockchain/pkg/core/consensus/blockgenerator/candidate"
	"github.com/dusk-network/dusk-blockchain/pkg/core/consensus/blockgenerator/score"
	"github.com/dusk-network/dusk-blockchain/pkg/core/data/ipc/keys"
	"github.com/dusk-network/dusk-blockchain/pkg/core/database"
)

type scoreGenerator = score.Generator
type candidateGenerator = candidate.Generator

type blockGenerator struct {
	scoreGenerator
	candidateGenerator
}

// BlockGenerator ...
type BlockGenerator interface {
	scoreGenerator
	candidateGenerator
}

// New creates a new BlockGenerator
func New(e *consensus.Emitter, genPubKey *keys.PublicKey, db database.DB) (BlockGenerator, error) {
	s, err := score.New(e, db)
	if err != nil {
		return nil, err
	}
	c := candidate.New(e, genPubKey)

	return &blockGenerator{
		scoreGenerator:     s,
		candidateGenerator: c,
	}, nil
}

// Mock the block generator. If inert is true, no block will be generated (this
// simulates the score not reaching the threshold)
func Mock(e *consensus.Emitter, inert bool) BlockGenerator {
	return &blockGenerator{
		scoreGenerator:     score.Mock(e, inert),
		candidateGenerator: candidate.Mock(e),
	}
}
