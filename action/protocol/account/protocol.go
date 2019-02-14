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
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

// ProtocolID is the protocol ID
// TODO: it works only for one instance per protocol definition now
const ProtocolID = "account"

// Protocol defines the protocol of handling account
type Protocol struct{}

// NewProtocol instantiates the protocol of account
func NewProtocol() *Protocol { return &Protocol{} }

// Handle handles an account
func (p *Protocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	switch act := act.(type) {
	case *action.Transfer:
		raCtx, ok := protocol.GetRunActionsCtx(ctx)
		if !ok {
			log.S().Panic("Miss run action context")
		}
		if err := p.handleTransfer(raCtx, act, sm); err != nil {
			return nil, errors.Wrap(err, "error when handling transfer action")
		}
	}
	return nil, nil
}

// Validate validates an account
func (p *Protocol) Validate(ctx context.Context, act action.Action) error {
	switch act := act.(type) {
	case *action.Transfer:
		if err := p.validateTransfer(ctx, act); err != nil {
			return errors.Wrap(err, "error when validating transfer action")
		}
	}
	return nil
}

// LoadOrCreateAccount either loads an account state or creates an account state
func LoadOrCreateAccount(sm protocol.StateManager, encodedAddr string, init *big.Int) (*state.Account, error) {
	var account state.Account
	addr, err := address.FromString(encodedAddr)
	if err != nil {
		account = state.EmptyAccount()
		return &account, errors.Wrap(err, "failed to get address public key hash from encoded address")
	}
	addrHash := byteutil.BytesTo20B(addr.Bytes())
	err = sm.State(addrHash, &account)
	if err == nil {
		return &account, nil
	}
	if errors.Cause(err) == state.ErrStateNotExist {
		account.Balance = init
		account.VotingWeight = big.NewInt(0)
		if err := sm.PutState(addrHash, account); err != nil {
			return nil, errors.Wrapf(err, "failed to put state for account %x", addrHash)
		}
		return &account, nil
	}
	return nil, err
}

// LoadAccount loads an account state
func LoadAccount(sm protocol.StateManager, addrHash hash.Hash160) (*state.Account, error) {
	var account state.Account
	if err := sm.State(addrHash, &account); err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			account = state.EmptyAccount()
			return &account, nil
		}
		return nil, err
	}
	return &account, nil
}

// StoreAccount puts updated account state to trie
func StoreAccount(sm protocol.StateManager, encodedAddr string, account *state.Account) error {
	addr, err := address.FromString(encodedAddr)
	if err != nil {
		return errors.Wrap(err, "failed to get address public key hash from encoded address")
	}
	addrHash := byteutil.BytesTo20B(addr.Bytes())
	return sm.PutState(addrHash, account)
}

// Recorded tests if an account has been actually stored
func Recorded(sm protocol.StateManager, addr address.Address) (bool, error) {
	var account state.Account
	err := sm.State(byteutil.BytesTo20B(addr.Bytes()), &account)
	if err == nil {
		return true, nil
	}
	if errors.Cause(err) == state.ErrStateNotExist {
		return false, nil
	}
	return false, err
}
