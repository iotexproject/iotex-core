// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package keystore

import (
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/pkg/keypair"
)

var (
	// ErrTransfer indicates the error of transfer
	ErrTransfer = errors.New("transfer error")
	// ErrVote indicates the error of vote
	ErrVote = errors.New("vote error")
)

// AccountManager manages keystore based accounts
type AccountManager struct {
	keystore KeyStore
}

// NewPlainAccountManager creates a new account manager based on plain keystore
func NewPlainAccountManager(dir string) (*AccountManager, error) {
	ksDir, _ := filepath.Abs(dir)
	ks, err := NewPlainKeyStore(ksDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a new plain keystore")
	}
	accountManager := &AccountManager{keystore: ks}
	return accountManager, nil
}

// NewMemAccountManager creates a new account manager based on in-memory keystore
func NewMemAccountManager() *AccountManager {
	accountManager := &AccountManager{
		keystore: NewMemKeyStore(),
	}
	return accountManager
}

// Contains returns whether keystore contains the given account
func (m *AccountManager) Contains(encodedAddr string) (bool, error) {
	return m.keystore.Has(encodedAddr)
}

// Remove removes the given account if exists
func (m *AccountManager) Remove(encodedAddr string) error {
	return m.keystore.Remove(encodedAddr)
}

// NewAccount creates and stores a new account
func (m *AccountManager) NewAccount() (keypair.PrivateKey, error) {
	pk, sk, err := crypto.EC283.NewKeyPair()
	if err != nil {
		return keypair.ZeroPrivateKey, errors.Wrap(err, "failed to generate key pair")
	}
	pkHash := keypair.HashPubKey(pk)
	// TODO: need to fix the chain ID
	addr := address.New(config.Default.Chain.ID, pkHash[:])
	if err := m.keystore.Store(addr.Bech32(), sk); err != nil {
		return keypair.ZeroPrivateKey, errors.Wrapf(err, "failed to store account %s", addr.Bech32())
	}
	return sk, nil
}

// Import imports key bytes and stores it as a new account
func (m *AccountManager) Import(keyBytes []byte) error {
	priKey, err := keypair.BytesToPrivateKey(keyBytes)
	if err != nil {
		return errors.Wrap(err, "failed to convert bytes to private key")
	}
	pubKey, err := crypto.EC283.NewPubKey(priKey)
	if err != nil {
		return errors.Wrap(err, "failed to derive public key from private key")
	}
	pkHash := keypair.HashPubKey(pubKey)
	addr := address.New(config.Default.Chain.ID, pkHash[:])
	if err := m.keystore.Store(addr.Bech32(), priKey); err != nil {
		return errors.Wrapf(err, "failed to store account %s", addr.Bech32())
	}
	return nil
}

// SignAction signs an action envelope.
func (m *AccountManager) SignAction(encodedAddr string, elp action.Envelope) (action.SealedEnvelope, error) {
	key, err := m.keystore.Get(encodedAddr)
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrapf(err, "failed to get the private key of account %s", encodedAddr)
	}
	selp, err := action.Sign(elp, encodedAddr, key)
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrapf(err, "failed to sign transfer %v", elp)
	}
	return selp, nil
}

// SignHash signs a hash
func (m *AccountManager) SignHash(encodedAddr string, hash []byte) ([]byte, error) {
	key, err := m.keystore.Get(encodedAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get the private key of account %s", encodedAddr)
	}
	return crypto.EC283.Sign(key, hash), nil
}
