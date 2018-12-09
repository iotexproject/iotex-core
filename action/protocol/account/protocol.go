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
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state"
)

// Protocol defines the protocol of handling account
type Protocol struct{}

// NewProtocol instantiates the protocol of account
func NewProtocol() *Protocol { return &Protocol{} }

// Handle handles an account
func (p *Protocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	raCtx, ok := state.GetRunActionsCtx(ctx)
	if !ok {
		return nil, errors.New("failed to get action context")
	}
	switch act := act.(type) {
	case *action.Transfer:
		if err := p.handleTransfer(act, raCtx, sm); err != nil {
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

// LoadOrCreateAccount either loads an account state or creates an account state
func LoadOrCreateAccount(sm protocol.StateManager, addr string, init *big.Int) (*state.Account, error) {
	addrHash, err := iotxaddress.AddressToPKHash(addr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert address to public key hash")
	}
	account, err := LoadAccount(sm, addrHash)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get account of %x from account trie", addrHash)
	}
	if account == state.EmptyAccount {
		account = &state.Account{
			Balance:      init,
			VotingWeight: big.NewInt(0),
		}
		if err := sm.PutState(addrHash, account); err != nil {
			return nil, errors.Wrapf(err, "failed to put state for account %x", addrHash)
		}
	}
	return account, nil
}

// LoadAccount loads an account state
func LoadAccount(sm protocol.StateManager, addrHash hash.PKHash) (*state.Account, error) {
	var s state.Account
	if err := sm.State(addrHash, &s); err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			return state.EmptyAccount, nil
		}
		return nil, err
	}
	return &s, nil
}

// StoreAccount puts updated account state to trie
func StoreAccount(sm protocol.StateManager, addr string, acct *state.Account) error {
	addrHash, err := iotxaddress.AddressToPKHash(addr)
	if err != nil {
		return errors.Wrap(err, "failed to convert address to public key hash")
	}
	return sm.PutState(addrHash, acct)
}

// SetNonce sets nonce for account
func SetNonce(act action.Action, state *state.Account) {
	if act.Nonce() > state.Nonce {
		state.Nonce = act.Nonce()
	}
}
