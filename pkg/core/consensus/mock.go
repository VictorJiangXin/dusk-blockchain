package consensus

import (
	"github.com/dusk-network/dusk-blockchain/pkg/core/consensus/key"
	"github.com/dusk-network/dusk-blockchain/pkg/core/consensus/user"
	crypto "github.com/dusk-network/dusk-crypto/hash"
)

// MockRoundUpdate mocks a round update
func MockRoundUpdate(round uint64, p *user.Provisioners, bidList user.BidList) RoundUpdate {
	var provisioners = p
	if p == nil {
		provisioners, _ = MockProvisioners(1)
	}

	var bidders = bidList
	if bidList == nil || len(bidList) == 0 {
		bidders = MockBidList(1)
	}

	seed, _ := crypto.RandEntropy(33)
	hash, _ := crypto.RandEntropy(32)
	return RoundUpdate{
		Round:   round,
		P:       *provisioners,
		BidList: bidders,
		Seed:    seed,
		Hash:    hash,
	}
}

//func MockRoundUpdateBuffer(round uint64, p *user.Provisioners, bidList user.BidList) *bytes.Buffer {
//	init := make([]byte, 8)
//	binary.LittleEndian.PutUint64(init, round)
//	buf := bytes.NewBuffer(init)
//
//	if p == nil {
//		p, _ = MockProvisioners(1)
//	}
//	user.MarshalProvisioners(buf, p)
//
//	if bidList == nil {
//		bidList = MockBidList(1)
//	}
//	user.MarshalBidList(buf, bidList)
//
//	seed, _ := crypto.RandEntropy(33)
//	if err := encoding.WriteBLS(buf, seed); err != nil {
//		panic(err)
//	}
//
//	hash, _ := crypto.RandEntropy(32)
//	if err := encoding.Write256(buf, hash); err != nil {
//		panic(err)
//	}
//
//	return buf
//}

// MockProvisioners mock a Provisioner set
func MockProvisioners(amount int) (*user.Provisioners, []key.Keys) {
	p := user.NewProvisioners()

	k := make([]key.Keys, amount)
	for i := 0; i < amount; i++ {
		keys, _ := key.NewRandKeys()
		member := MockMember(keys)

		p.Members[string(keys.BLSPubKeyBytes)] = member
		p.Set.Insert(keys.BLSPubKeyBytes)
		k[i] = keys
	}
	return p, k
}

// MockMember mocks a Provisioner
func MockMember(keys key.Keys) *user.Member {
	member := &user.Member{}
	member.PublicKeyEd = keys.EdPubKeyBytes
	member.PublicKeyBLS = keys.BLSPubKeyBytes
	member.Stakes = make([]user.Stake, 1)
	member.Stakes[0].Amount = 500
	member.Stakes[0].EndHeight = 10000
	return member
}

// MockBidList mocks a bid list
func MockBidList(amount int) user.BidList {
	bidList := make([]user.Bid, amount)
	for i := 0; i < amount; i++ {
		xSlice, _ := crypto.RandEntropy(32)
		mSlice, _ := crypto.RandEntropy(32)
		var x [32]byte
		var m [32]byte
		copy(x[:], xSlice)
		copy(m[:], mSlice)
		bidList[i] = user.Bid{X: x, M: m, EndHeight: 10}
	}
	return bidList
}
