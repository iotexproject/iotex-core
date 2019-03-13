// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mainchain

import (
	"context"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

// ProtocolID is the protocol ID
// TODO: it works only for one instance per protocol definition now
const ProtocolID = "multi-chain_main-chain"

var (
	// MinSecurityDeposit represents the security deposit minimal required for start a sub-chain, which is 1M iotx
	MinSecurityDeposit = big.NewInt(0).Mul(big.NewInt(1000000), big.NewInt(unit.Iotx))
	// SubChainsInOperationKey is to find the used chain IDs in the state factory
	// TODO: this is a not safe way to define the key, as other protocols could collide it
	SubChainsInOperationKey = hash.Hash160b([]byte("subChainsInOperation"))
)

// Protocol defines the protocol of handling multi-chain actions on main-chain
type Protocol struct {
	rootChain blockchain.Blockchain
	sf        factory.Factory
}

// NewProtocol instantiates the protocol of sub-chain
func NewProtocol(rootChain blockchain.Blockchain) *Protocol {
	return &Protocol{
		rootChain: rootChain,
		sf:        rootChain.GetFactory(),
	}
}

// Handle handles how to mutate the state db given the multi-chain action on main-chain
func (p *Protocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	switch act := act.(type) {
	case *action.StartSubChain:
		if err := p.handleStartSubChain(ctx, act, sm); err != nil {
			return nil, errors.Wrapf(err, "error when handling start sub-chain action")
		}
	case *action.PutBlock:
		if err := p.handlePutBlock(ctx, act, sm); err != nil {
			return nil, errors.Wrapf(err, "error when handling put sub-chain block action")
		}
	case *action.CreateDeposit:
		deposit, err := p.handleDeposit(ctx, act, sm)
		if err != nil {
			return nil, errors.Wrapf(err, "error when handling deposit creation action")
		}
		return deposit, nil
	case *action.StopSubChain:
		if err := p.handleStopSubChain(ctx, act, sm); err != nil {
			return nil, errors.Wrapf(err, "error when handling stop sub-chain action")
		}
	}
	// The action is not handled by this handler or no error
	return nil, nil
}

// Validate validates the multi-chain action on main-chain
func (p *Protocol) Validate(ctx context.Context, act action.Action) error {
	switch act := act.(type) {
	case *action.StartSubChain:
		vaCtx := protocol.MustGetValidateActionsCtx(ctx)
		if _, _, err := p.validateStartSubChain(vaCtx.Caller, act, nil); err != nil {
			return errors.Wrapf(err, "error when validating start sub-chain action")
		}
	case *action.PutBlock:
		if err := p.validatePutBlock(act, nil); err != nil {
			return errors.Wrapf(err, "error when validating put sub-chain block action")
		}
	case *action.CreateDeposit:
		vaCtx := protocol.MustGetValidateActionsCtx(ctx)
		if _, _, err := p.validateDeposit(vaCtx.Caller, act, nil); err != nil {
			return errors.Wrapf(err, "error when validating deposit creation action")
		}
	}
	// The action is not validated by this handler or no error
	return nil
}

// ReadState read the state on blockchain via protocol
func (p *Protocol) ReadState(context.Context, protocol.StateManager, []byte, ...[]byte) ([]byte, error) {
	return nil, protocol.ErrUnimplemented
}

func (p *Protocol) account(sender string, sm protocol.StateManager) (*state.Account, error) {
	if sm == nil {
		return p.sf.AccountState(sender)
	}
	addr, err := address.FromString(sender)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert address to public key hash")
	}
	addrHash := hash.BytesToHash160(addr.Bytes())
	return accountutil.LoadAccount(sm, addrHash)
}

func (p *Protocol) accountWithEnoughBalance(
	sender string,
	balance *big.Int,
	sm protocol.StateManager,
) (*state.Account, error) {
	account, err := p.account(sender, sm)
	if err != nil {
		return nil, errors.Wrapf(err, "error when getting the account of address %s", sender)
	}
	if account.Balance.Cmp(balance) < 0 {
		return nil, errors.Errorf("%s doesn't have at least required balance %d", sender, balance)
	}
	return account, nil
}

func (p *Protocol) subChainsInOperation(sm protocol.StateManager) (SubChainsInOperation, error) {
	var subChainsInOp SubChainsInOperation
	var err error
	if sm == nil {
		subChainsInOp, err = p.SubChainsInOperation()
	} else {
		err = sm.State(SubChainsInOperationKey, &subChainsInOp)
		if err != nil && errors.Cause(err) == state.ErrStateNotExist {
			err = nil
		}
	}
	if err != nil {
		return nil, errors.Wrap(err, "error when getting the state of sub-chains in operation")
	}
	return subChainsInOp, nil
}

func srcAddressPKHash(srcAddr string) (hash.Hash160, error) {
	addr, err := address.FromString(srcAddr)
	if err != nil {
		return hash.ZeroHash160, errors.Wrapf(err, "cannot get the public key hash of address %s", srcAddr)
	}
	return hash.BytesToHash160(addr.Bytes()), nil
}

// SubChain returns the confirmed sub-chain state
func (p *Protocol) SubChain(addr address.Address) (*SubChain, error) {
	var subChain SubChain
	if err := p.sf.State(hash.BytesToHash160(addr.Bytes()), &subChain); err != nil {
		return nil, errors.Wrapf(err, "error when loading state of %x", addr.Bytes())
	}
	return &subChain, nil
}

// SubChainsInOperation returns the used chain IDs
func (p *Protocol) SubChainsInOperation() (SubChainsInOperation, error) {
	var subChainsInOp SubChainsInOperation
	err := p.sf.State(SubChainsInOperationKey, &subChainsInOp)
	if err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return nil, err
	}
	return subChainsInOp, nil
}
