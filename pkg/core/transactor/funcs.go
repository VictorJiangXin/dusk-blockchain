package transactor

import (
	"bytes"
	"context"

	cfg "github.com/dusk-network/dusk-blockchain/pkg/config"
	walletdb "github.com/dusk-network/dusk-blockchain/pkg/core/data/database"
	"github.com/dusk-network/dusk-blockchain/pkg/core/data/ipc/common"
	"github.com/dusk-network/dusk-blockchain/pkg/core/data/ipc/keys"
	"github.com/dusk-network/dusk-blockchain/pkg/core/data/wallet"
)

var testnet = byte(2)

func (t *Transactor) createWallet(seed []byte, password string) error {
	// First load the database
	db, err := walletdb.New(cfg.Get().Wallet.Store)
	if err != nil {
		return err
	}

	if seed == nil {
		seed, err = wallet.GenerateNewSeed(nil)
		if err != nil {
			return err
		}
	}

	sk, pk, vk, err := t.keyMaster.GenerateKeys(context.Background(), seed)
	if err != nil {
		return err
	}

	t.secretKey = sk

	skBuf := new(bytes.Buffer)
	if err = keys.MarshalSecretKey(skBuf, &t.secretKey); err != nil {
		_ = db.Close()
		return err
	}

	keysJSON := wallet.KeysJSON{
		Seed:      seed,
		SecretKey: skBuf.Bytes(),
		PublicKey: pk,
		ViewKey:   vk,
	}

	// Then create the wallet with seed and password
	w, err := wallet.LoadFromSeed(testnet, db, password, cfg.Get().Wallet.File, keysJSON)
	if err != nil {
		_ = db.Close()
		return err
	}

	// assign wallet
	t.w = w

	return nil
}

func (t *Transactor) loadWallet(password string) (pubKey keys.PublicKey, err error) {
	// First load the database
	db, err := walletdb.New(cfg.Get().Wallet.Store)
	if err != nil {
		return pubKey, err
	}

	// Then load the wallet
	w, err := wallet.LoadFromFile(testnet, db, password, cfg.Get().Wallet.File)
	if err != nil {
		_ = db.Close()
		return pubKey, err
	}

	// assign wallet
	t.w = w

	return w.PublicKey, err
}

// DecodeAddressToPublicKey will decode a []byte to rusk.PublicKey
func DecodeAddressToPublicKey(in []byte) (*keys.StealthAddress, error) {
	pk := keys.NewStealthAddress()
	var buf = &bytes.Buffer{}
	_, err := buf.Write(in)
	if err != nil {
		return pk, err
	}

	pk.RG = new(common.JubJubCompressed)
	pk.PkR = new(common.JubJubCompressed)
	pk.RG.Data = make([]byte, 32)
	pk.PkR.Data = make([]byte, 32)

	if _, err = buf.Read(pk.RG.Data); err != nil {
		return pk, err
	}

	if _, err = buf.Read(pk.PkR.Data); err != nil {
		return pk, err
	}

	return pk, nil
}
