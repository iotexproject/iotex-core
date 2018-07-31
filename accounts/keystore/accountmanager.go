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

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
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
	addr, err := iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate a new iotxaddress")
	}
	if err := m.keystore.Store(addr.RawAddress, addr); err != nil {
		return nil, errors.Wrapf(err, "failed to store account %s", addr.RawAddress)
	}
	return addr, nil
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
func (m *AccountManager) SignTransfer(rawAddr string, rawTransfer *action.Transfer) (*action.Transfer, error) {
	if rawTransfer == nil {
		return nil, errors.Wrap(ErrTransfer, "transfer cannot be nil")
	}
	addr, err := m.keystore.Get(rawAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get account %s", rawAddr)
	}
	signedTransfer, err := rawTransfer.Sign(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to sign transfer %v", rawTransfer)
	}
	return signedTransfer, nil
}

// SignVote signs a vote
func (m *AccountManager) SignVote(rawAddr string, rawVote *action.Vote) (*action.Vote, error) {
	if rawVote == nil {
		return nil, errors.Wrap(ErrVote, "vote cannot be nil")
	}
	addr, err := m.keystore.Get(rawAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get account %s", rawAddr)
	}
	signedVote, err := rawVote.Sign(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to sign vote %v", rawVote)
	}
	return signedVote, nil
}

// SignHash signs a hash
func (m *AccountManager) SignHash(rawAddr string, hash []byte) ([]byte, error) {
	addr, err := m.keystore.Get(rawAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get account %s", rawAddr)
	}
	return crypto.Sign(addr.PrivateKey, hash), nil
}
