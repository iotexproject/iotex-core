// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package keystore

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
)

var (
	// ErrNilAccountManager indicates that account manager is nil
	ErrNilAccountManager = errors.New("account manager is nil")
	// ErrNumAccounts indicates invalid number of accounts in keystore
	ErrNumAccounts = errors.New("number of accounts is invalid")
)

// SingleAccountManager is a special account manager that maintains a single account in keystore
type SingleAccountManager struct {
	accountManager *AccountManager
}

// NewSingleAccountManager creates a new single account manager
func NewSingleAccountManager(accountManager *AccountManager) (*SingleAccountManager, error) {
	if accountManager == nil {
		return nil, errors.Wrap(ErrNilAccountManager, "try to attach to a nil account manager")
	}
	accounts, err := accountManager.keystore.All()
	if err != nil {
		return nil, errors.Wrap(err, "failed to list all accounts")
	}
	if len(accounts) != 1 {
		return nil, errors.Wrap(ErrNumAccounts, "only one account is allowed in keystore")
	}
	singleAccountManager := &SingleAccountManager{accountManager: accountManager}
	return singleAccountManager, nil
}

// SignAction signs an action envelope.
func (m *SingleAccountManager) SignAction(elp action.Envelope) (action.SealedEnvelope, error) {
	accounts, err := m.accountManager.keystore.All()
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrap(err, "failed to list all accounts")
	}
	if len(accounts) != 1 {
		return action.SealedEnvelope{}, errors.Wrap(ErrNumAccounts, "only one account is allowed in keystore")
	}
	return m.accountManager.SignAction(accounts[0], elp)
}

// SignHash signs a hash
func (m *SingleAccountManager) SignHash(hash []byte) ([]byte, error) {
	accounts, err := m.accountManager.keystore.All()
	if err != nil {
		return nil, errors.Wrap(err, "failed to list all accounts")
	}
	if len(accounts) != 1 {
		return nil, errors.Wrap(ErrNumAccounts, "only one account is allowed in keystore")
	}
	return m.accountManager.SignHash(accounts[0], hash)
}
