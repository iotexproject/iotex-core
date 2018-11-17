// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"context"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

// Protocol defines the protocol of handling account
type Protocol struct{}

// NewProtocol instantiates the protocol of account
func NewProtocol() *Protocol { return &Protocol{} }

// Handle handles an account
func (p *Protocol) Handle(_ context.Context, act action.Action, ws factory.WorkingSet) (*action.Receipt, error) {
	switch act := act.(type) {
	case *action.Transfer:
		if err := p.handleTransfer(act, ws); err != nil {
			return nil, errors.Wrap(err, "error when handling transfer action")
		}
	}
	return nil, nil
}

// Validate validates an account
func (p *Protocol) Validate(_ context.Context, act action.Action) error {
	switch act := act.(type) {
	case *action.Transfer:
		if err := p.validateTransfer(act); err != nil {
			return errors.Wrap(err, "error when validating transfer action")
		}
	}
	return nil
}

// LoadOrCreateAccountState either loads an account state or creates an account state
func LoadOrCreateAccountState(ws factory.WorkingSet, addr string, init *big.Int) (*state.Account, error) {
	addrHash, err := iotxaddress.AddressToPKHash(addr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert address to public key hash")
	}
	account, err := LoadAccountState(ws, addrHash)
	switch {
	case errors.Cause(err) == factory.ErrStateNotExist:
		account := state.Account{
			Balance:      init,
			VotingWeight: big.NewInt(0),
		}
		if err := ws.PutState(addrHash, &account); err != nil {
			return nil, errors.Wrapf(err, "failed to put state for account %x", addrHash)
		}
		return &account, nil
	case err != nil:
		return nil, errors.Wrapf(err, "failed to get account of %x from account trie", addrHash)
	}
	return account, nil
}

// LoadAccountState loads an account state
func LoadAccountState(ws factory.WorkingSet, addrHash hash.PKHash) (*state.Account, error) {
	var s state.Account
	if err := ws.State(addrHash, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// StoreState put updated state to trie
func StoreState(ws factory.WorkingSet, addr string, state state.State) error {
	addrHash, err := iotxaddress.AddressToPKHash(addr)
	if err != nil {
		return errors.Wrap(err, "failed to convert address to public key hash")
	}
	return ws.PutState(addrHash, state)
}

// SetNonce sets nonce for account
func SetNonce(act action.Action, state *state.Account) {
	if act.Nonce() > state.Nonce {
		state.Nonce = act.Nonce()
	}
}
