// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package accountutil

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state"
)

type noncer interface {
	Nonce() uint64
}

// SetNonce sets nonce for account
func SetNonce(i noncer, state *state.Account) {
	if i.Nonce() >= state.PendingNonce() {
		if err := state.SetNonce(i.Nonce()); err != nil {
			panic("invalid nonce")
		}
	}
}

// LoadOrCreateAccount either loads an account state or creates an account state
func LoadOrCreateAccount(sm protocol.StateManager, addr address.Address, opts ...state.AccountCreationOption) (*state.Account, error) {
	var (
		account  = state.NewEmptyAccount()
		addrHash = hash.BytesToHash160(addr.Bytes())
	)
	_, err := sm.State(account, protocol.LegacyKeyOption(addrHash))
	switch errors.Cause(err) {
	case nil:
		return account, nil
	case state.ErrStateNotExist:
		account, err := state.NewAccount(opts...)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create state account for %x", addrHash)
		}
		if _, err := sm.PutState(account, protocol.LegacyKeyOption(addrHash)); err != nil {
			return nil, errors.Wrapf(err, "failed to put state for account %x", addrHash)
		}
		return account, nil
	default:
		return nil, err
	}
}

// LoadAccount loads an account state by address.Address
func LoadAccount(sr protocol.StateReader, addr address.Address, opts ...state.AccountCreationOption) (*state.Account, error) {
	return LoadAccountByHash160(sr, hash.BytesToHash160(addr.Bytes()), opts...)
}

// LoadAccountByHash160 loads an account state by 20-byte address
func LoadAccountByHash160(sr protocol.StateReader, addrHash hash.Hash160, opts ...state.AccountCreationOption) (*state.Account, error) {
	account := state.NewEmptyAccount()
	switch _, err := sr.State(account, protocol.LegacyKeyOption(addrHash)); errors.Cause(err) {
	case state.ErrStateNotExist:
		return state.NewAccount(opts...)
	case nil:
		return account, nil
	default:
		return nil, err
	}
}

// StoreAccount puts updated account state to trie
func StoreAccount(sm protocol.StateManager, addr address.Address, account *state.Account) error {
	addrHash := hash.BytesToHash160(addr.Bytes())
	_, err := sm.PutState(account, protocol.LegacyKeyOption(addrHash))
	return err
}

// Recorded tests if an account has been actually stored
func Recorded(sr protocol.StateReader, addr address.Address) (bool, error) {
	account := state.NewEmptyAccount()
	_, err := sr.State(account, protocol.LegacyKeyOption(hash.BytesToHash160(addr.Bytes())))
	switch errors.Cause(err) {
	case nil:
		return true, nil
	case state.ErrStateNotExist:
		return false, nil
	}
	return false, err
}

// AccountState returns the confirmed account state on the chain
func AccountState(sr protocol.StateReader, addr address.Address) (*state.Account, error) {
	a, _, err := AccountStateWithHeight(sr, addr)
	return a, err
}

// AccountStateWithHeight returns the confirmed account state on the chain with what height the state is read from.
func AccountStateWithHeight(sr protocol.StateReader, addr address.Address) (*state.Account, uint64, error) {
	pkHash := hash.BytesToHash160(addr.Bytes())
	account := state.NewEmptyAccount()
	h, err := sr.State(account, protocol.LegacyKeyOption(pkHash))
	switch errors.Cause(err) {
	case nil:
		fallthrough
	case state.ErrStateNotExist:
		return account, h, nil
	default:
		return nil, h, errors.Wrapf(err, "error when loading state of %x", pkHash)
	}
}
