// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package keystore

import (
	"encoding/json"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
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
func (m *AccountManager) Contains(rawAddr string) (bool, error) {
	return m.keystore.Has(rawAddr)
}

// Remove removes the given account if exists
func (m *AccountManager) Remove(rawAddr string) error {
	return m.keystore.Remove(rawAddr)
}

// NewAccount creates a new account
func (m *AccountManager) NewAccount() (*iotxaddress.Address, error) {
	pk, sk, err := crypto.EC283.NewKeyPair()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate key pair")
	}
	// TODO: need to fix the chain ID
	pkHash := keypair.HashPubKey(pk)
	addr := address.New(config.Default.Chain.ID, pkHash[:])
	iotxAddr := iotxaddress.Address{
		PublicKey:  pk,
		PrivateKey: sk,
		RawAddress: addr.IotxAddress(),
	}
	if err := m.keystore.Store(iotxAddr.RawAddress, &iotxAddr); err != nil {
		return nil, errors.Wrapf(err, "failed to store account %s", iotxAddr.RawAddress)
	}
	return &iotxAddr, nil
}

// Import imports key bytes and stores it as a new account
func (m *AccountManager) Import(keyBytes []byte) error {
	var key Key
	if err := json.Unmarshal(keyBytes, &key); err != nil {
		return errors.Wrap(err, "failed to unmarshal key")
	}
	address, err := keyToAddr(&key)
	if err != nil {
		return errors.Wrap(err, "fail to convert key to address")
	}
	if err := m.keystore.Store(address.RawAddress, address); err != nil {
		return errors.Wrapf(err, "failed to store account %s", address.RawAddress)
	}
	return nil
}

// SignTransfer signs a transfer
func (m *AccountManager) SignTransfer(rawAddr string, transfer *action.Transfer) error {
	if transfer == nil {
		return errors.Wrap(ErrTransfer, "transfer cannot be nil")
	}
	addr, err := m.keystore.Get(rawAddr)
	if err != nil {
		return errors.Wrapf(err, "failed to get account %s", rawAddr)
	}
	if err := action.Sign(transfer, addr.PrivateKey); err != nil {
		return errors.Wrapf(err, "failed to sign transfer %v", transfer)
	}
	return nil
}

// SignVote signs a vote
func (m *AccountManager) SignVote(rawAddr string, vote *action.Vote) error {
	if vote == nil {
		return errors.Wrap(ErrVote, "vote cannot be nil")
	}
	addr, err := m.keystore.Get(rawAddr)
	if err != nil {
		return errors.Wrapf(err, "failed to get account %s", rawAddr)
	}
	if err := action.Sign(vote, addr.PrivateKey); err != nil {
		return errors.Wrapf(err, "failed to sign vote %v", vote)
	}
	return nil
}

// SignHash signs a hash
func (m *AccountManager) SignHash(rawAddr string, hash []byte) ([]byte, error) {
	addr, err := m.keystore.Get(rawAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get account %s", rawAddr)
	}
	return crypto.EC283.Sign(addr.PrivateKey, hash), nil
}
