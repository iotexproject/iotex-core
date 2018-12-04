// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mainchain

import (
	"context"
	"fmt"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

var (
	// MinSecurityDeposit represents the security deposit minimal required for start a sub-chain, which is 1M iotx
	MinSecurityDeposit = big.NewInt(0).Mul(big.NewInt(1000000000), big.NewInt(blockchain.Iotx))
	// SubChainsInOperationKey is to find the used chain IDs in the state factory
	// TODO: this is a not safe way to define the key, as other protocols could collide it
	SubChainsInOperationKey = byteutil.BytesTo20B(hash.Hash160b([]byte("subChainsInOperation")))
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
func (p *Protocol) Handle(_ context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	switch act := act.(type) {
	case *action.StartSubChain:
		if err := p.handleStartSubChain(act, sm); err != nil {
			return nil, errors.Wrapf(err, "error when handling start sub-chain action")
		}
	case *action.PutBlock:
		if err := p.handlePutBlock(act, sm); err != nil {
			return nil, errors.Wrapf(err, "error when handling put sub-chain block action")
		}
	case *action.CreateDeposit:
		deposit, err := p.handleDeposit(act, sm)
		if err != nil {
			return nil, errors.Wrapf(err, "error when handling deposit creation action")
		}
		return deposit, nil
	case *action.StopSubChain:
		if err := p.handleStopSubChain(act, sm); err != nil {
			return nil, errors.Wrapf(err, "error when handling stop sub-chain action")
		}
	}
	// The action is not handled by this handler or no error
	return nil, nil
}

// Validate validates the multi-chain action on main-chain
func (p *Protocol) Validate(_ context.Context, act action.Action) error {
	switch act := act.(type) {
	case *action.StartSubChain:
		if _, _, err := p.validateStartSubChain(act, nil); err != nil {
			return errors.Wrapf(err, "error when validating start sub-chain action")
		}
	case *action.PutBlock:
		if err := p.validatePutBlock(act, nil); err != nil {
			return errors.Wrapf(err, "error when validating put sub-chain block action")
		}
	case *action.CreateDeposit:
		if _, _, err := p.validateDeposit(act, nil); err != nil {
			return errors.Wrapf(err, "error when validating deposit creation action")
		}
	}
	// The action is not validated by this handler or no error
	return nil
}

func (p *Protocol) account(sender string, sm protocol.StateManager) (*state.Account, error) {
	if sm == nil {
		return p.sf.AccountState(sender)
	}
	addrHash, err := iotxaddress.AddressToPKHash(sender)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert address to public key hash")
	}
	return account.LoadAccountState(sm, addrHash)
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
		return nil, fmt.Errorf("%s doesn't have at least required balance %d", sender, balance)
	}
	return account, nil
}

func (p *Protocol) subChainsInOperation(sm protocol.StateManager) (state.SortedSlice, error) {
	var subChainsInOp state.SortedSlice
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
		return state.SortedSlice{}, errors.Wrap(err, "error when getting the state of sub-chains in operation")
	}
	return subChainsInOp, nil
}

func srcAddressPKHash(srcAddr string) (hash.PKHash, error) {
	addr, err := address.IotxAddressToAddress(srcAddr)
	if err != nil {
		return hash.ZeroPKHash, errors.Wrapf(err, "cannot get the public key hash of address %s", srcAddr)
	}
	return byteutil.BytesTo20B(addr.Payload()), nil
}

// SubChain returns the confirmed sub-chain state
func (p *Protocol) SubChain(addr address.Address) (*SubChain, error) {
	var subChain SubChain
	if err := p.sf.State(byteutil.BytesTo20B(addr.Payload()), &subChain); err != nil {
		return nil, errors.Wrapf(err, "error when loading state of %x", addr.Payload())
	}
	return &subChain, nil
}

// SubChainsInOperation returns the used chain IDs
func (p *Protocol) SubChainsInOperation() (state.SortedSlice, error) {
	var subChainsInOp state.SortedSlice
	err := p.sf.State(SubChainsInOperationKey, &subChainsInOp)
	if err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return nil, err
	}
	return subChainsInOp, nil
}
