package consensus

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"time"

	"gitlab.dusk.network/dusk-core/dusk-go/pkg/crypto/hash"
	"gitlab.dusk.network/dusk-core/dusk-go/pkg/p2p/wire/payload"
)

func binaryAgreement(ctx *Context, empty bool, c chan *payload.MsgBinary) error {
	// Prepare empty block
	emptyBlock, err := payload.NewEmptyBlock(ctx.GetLastHeader())
	if err != nil {
		return err
	}

	for ctx.step = 1; ctx.step < ctx.MaxSteps; ctx.step++ {
		ctx.AdjustVarsAgreement()

		// Save our currently kept block hash
		var startHash []byte
		startHash = append(startHash, ctx.BlockHash...)

		var msgs []*payload.MsgBinary
		var err error
		if _, err := committeeVoteBinary(ctx); err != nil {
			return err
		}

		_, err = countVotesBinary(ctx, c)
		if err != nil {
			return err
		}

		// Increase timer leniency
		if ctx.step > 1 {
			ctx.Lambda = ctx.Lambda * 2
		}

		if ctx.BlockHash == nil {
			ctx.BlockHash = startHash
		}

		if ctx.empty {
			ctx.BlockHash = emptyBlock.Header.Hash
		} else if ctx.step == 1 {
			ctx.step = ctx.MaxSteps
			if _, err := committeeVoteBinary(ctx); err != nil {
				return err
			}

			return nil
		}

		ctx.step++
		ctx.AdjustVarsAgreement()
		if _, err := committeeVoteBinary(ctx); err != nil {
			return err
		}

		_, err = countVotesBinary(ctx, c)
		if ctx.BlockHash == nil {
			ctx.BlockHash = emptyBlock.Header.Hash
			empty = true
		}

		if ctx.empty {
			return nil
		}

		ctx.step++
		ctx.AdjustVarsAgreement()
		vote, err := committeeVoteBinary(ctx)
		if err != nil {
			return err
		}

		msgs, err = countVotesBinary(ctx, c)
		msgs = append(msgs, vote)
		if ctx.BlockHash == nil {
			result, err := commonCoin(ctx, msgs)
			if err != nil {
				return err
			}

			if result == 0 {
				ctx.BlockHash = startHash
				continue
			}

			ctx.BlockHash = emptyBlock.Header.Hash
		}
	}

	return nil
}

func commonCoin(ctx *Context, allMsgs []*payload.MsgBinary) (uint64, error) {
	var lenHash, _ = new(big.Int).SetString("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", 0)
	for i, vote := range allMsgs {
		votes, h, err := processMsgBinary(ctx, vote)
		if err != nil {
			return 0, err
		}

		for j := 1; j < votes; j++ {
			binary.LittleEndian.PutUint32(h, uint32(i))
			result, err := hash.Sha3256(h)
			if err != nil {
				return 0, err
			}

			resultInt := new(big.Int).SetBytes(result)
			if resultInt.Cmp(lenHash) == -1 {
				lenHash = resultInt
			}
		}
	}

	lenHash.Mod(lenHash, big.NewInt(2))
	return lenHash.Uint64(), nil
}

func committeeVoteBinary(ctx *Context) (*payload.MsgBinary, error) {
	role := &role{
		part:  "committee",
		round: ctx.Round,
		step:  ctx.step,
	}

	if err := sortition(ctx, role); err != nil {
		return nil, err
	}

	if ctx.votes > 0 {
		// Sign block hash with BLS
		sigBLS, err := ctx.BLSSign(ctx.Keys.BLSSecretKey, ctx.BlockHash)
		if err != nil {
			return nil, err
		}

		// Create message to sign with ed25519
		var edMsg []byte
		copy(edMsg, ctx.Score)
		binary.LittleEndian.PutUint64(edMsg, ctx.Round)
		edMsg = append(edMsg, byte(ctx.step))
		edMsg = append(edMsg, ctx.GetLastHeader().Hash...)
		edMsg = append(edMsg, sigBLS...)

		// Sign with ed25519
		sigEd, err := ctx.EDSign(ctx.Keys.EdSecretKey, edMsg)
		if err != nil {
			return nil, err
		}

		// Create binary message to gossip
		blsPubBytes, err := ctx.Keys.BLSPubKey.MarshalBinary()
		if err != nil {
			return nil, err
		}

		msg, err := payload.NewMsgBinary(ctx.Score, ctx.empty, ctx.BlockHash, ctx.GetLastHeader().Hash, sigEd,
			[]byte(*ctx.Keys.EdPubKey), sigBLS, blsPubBytes, ctx.W, ctx.Round, ctx.step)
		if err != nil {
			return nil, err
		}

		if err := ctx.SendMessage(ctx.Magic, msg); err != nil {
			return nil, err
		}

		// Return the message for inclusion in the common coin procedure
		return msg, nil
	}

	return nil, nil
}

func countVotesBinary(ctx *Context, c chan *payload.MsgBinary) ([]*payload.MsgBinary, error) {
	counts := make(map[string]int)
	var voters [][]byte
	var allMsgs []*payload.MsgBinary
	voters = append(voters, []byte(*ctx.Keys.EdPubKey))
	counts[hex.EncodeToString(ctx.BlockHash)] += ctx.votes
	timer := time.NewTimer(ctx.Lambda)

end:
	for {
		select {
		case <-timer.C:
			break end
		case m := <-c:
			// Verify the message score and get back it's contents
			votes, hash, err := processMsgBinary(ctx, m)
			if err != nil {
				return nil, err
			}

			// If votes is zero, then the reduction message was most likely
			// faulty, so we will ignore it.
			if votes == 0 {
				break
			}

			// Check if this node's vote is already recorded
			for _, voter := range voters {
				if bytes.Compare(voter, m.PubKeyEd) == 0 {
					break
				}
			}

			// Log new information
			voters = append(voters, m.PubKeyEd)
			hashStr := hex.EncodeToString(hash)
			counts[hashStr] += votes

			// Save vote for common coin
			allMsgs = append(allMsgs, m)

			// If a block exceeds the vote threshold, we will return it's hash
			// and end the loop.
			if counts[hashStr] > int(float64(ctx.VoteLimit)*ctx.Threshold) {
				timer.Stop()
				ctx.empty = m.Empty
				ctx.BlockHash = hash
				return allMsgs, nil
			}
		}
	}

	ctx.BlockHash = nil
	return allMsgs, nil
}

func processMsgBinary(ctx *Context, msg *payload.MsgBinary) (int, []byte, error) {
	// Verify message
	if !verifySignaturesBinary(ctx, msg) {
		return 0, nil, nil
	}

	role := &role{
		part:  "committee",
		round: ctx.Round,
		step:  ctx.step,
	}

	// Check if we're on the same chain
	if bytes.Compare(msg.PrevBlockHash, ctx.GetLastHeader().Hash) != 0 {
		// Either an old message or a malformed message
		return 0, nil, nil
	}

	// Make sure their score is valid, and calculate their amount of votes.
	votes, err := verifySortition(ctx, msg.Score, msg.PubKeyBLS, role, msg.Stake)
	if err != nil {
		return 0, nil, err
	}

	if votes == 0 {
		return 0, nil, nil
	}

	return votes, msg.BlockHash, nil
}

func verifySignaturesBinary(ctx *Context, msg *payload.MsgBinary) bool {
	// Construct message
	var edMsg []byte
	copy(edMsg, msg.Score)
	binary.LittleEndian.PutUint64(edMsg, msg.Round)
	edMsg = append(edMsg, byte(msg.Step))
	edMsg = append(edMsg, msg.PrevBlockHash...)
	edMsg = append(edMsg, msg.SigBLS...)

	// Check ed25519
	if !ctx.EDVerify(msg.PubKeyEd, edMsg, msg.SigEd) {
		return false
	}

	// Check BLS
	if err := ctx.BLSVerify(msg.PubKeyBLS, msg.BlockHash, msg.SigBLS); err != nil {
		return false
	}

	// Passed all checks
	return true
}