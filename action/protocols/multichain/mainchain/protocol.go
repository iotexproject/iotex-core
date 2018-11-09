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
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
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
	sf        state.Factory
}

// NewProtocol instantiates the protocol of sub-chain
func NewProtocol(rootChain blockchain.Blockchain) *Protocol {
	return &Protocol{
		rootChain: rootChain,
		sf:        rootChain.GetFactory(),
	}
}

// Handle handles how to mutate the state db given the multi-chain action on main-chain
func (p *Protocol) Handle(_ context.Context, act action.Action, ws state.WorkingSet) (*action.Receipt, error) {
	switch act := act.(type) {
	case *action.StartSubChain:
		if err := p.handleStartSubChain(act, ws); err != nil {
			return nil, errors.Wrapf(err, "error when handling start sub-chain action")
		}
	case *action.PutBlock:
		if err := p.handlePutBlock(act, ws); err != nil {
			return nil, errors.Wrapf(err, "error when handling put sub-chain block action")
		}
	case *action.CreateDeposit:
		deposit, err := p.handleDeposit(act, ws)
		if err != nil {
			return nil, errors.Wrapf(err, "error when handling deposit creation action")
		}
		return deposit, nil
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

func (p *Protocol) accountWithEnoughBalance(
	sender string,
	balance *big.Int,
	ws state.WorkingSet,
) (*state.Account, error) {
	var account *state.Account
	var err error
	if ws == nil {
		account, err = p.sf.AccountState(sender)
	} else {
		account, err = ws.CachedAccountState(sender)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "error when getting the account of address %s", sender)
	}
	if account.Balance.Cmp(balance) < 0 {
		return nil, fmt.Errorf("%s doesn't have at least required balance %d", sender, balance)
	}
	return account, nil
}

func (p *Protocol) subChainsInOperation(ws state.WorkingSet) (state.SortedSlice, error) {
	var subChainsInOp state.SortedSlice
	var err error
	if ws == nil {
		subChainsInOp, err = p.SubChainsInOperation()
	} else {
		subChainsInOp, err = processState(ws.State(SubChainsInOperationKey, &subChainsInOp))
	}
	if err != nil {
		return state.SortedSlice{}, errors.Wrap(err, "error when getting the state of sub-chains in operation")
	}
	return subChainsInOp, nil
}

func processState(s state.State, err error) (state.SortedSlice, error) {
	if err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			return state.SortedSlice{}, nil
		}
		return nil, errors.Wrapf(err, "error when loading state of %x", SubChainsInOperationKey)
	}
	uci, ok := s.(*state.SortedSlice)
	if !ok {
		return nil, errors.New("error when casting state into used chain IDs")
	}
	return *uci, nil
}

func srcAddressPKHash(srcAddr string) (hash.PKHash, error) {
	addr, err := address.IotxAddressToAddress(srcAddr)
	if err != nil {
		return hash.ZeroPKHash, errors.Wrapf(err, "cannot get the public key hash of address %s", srcAddr)
	}
	return byteutil.BytesTo20B(addr.Payload()), nil
}
