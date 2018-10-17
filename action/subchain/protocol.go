// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package subchain

import (
	"fmt"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/state"
)

const (
	// MainChainID reserves the ID for main chain
	MainChainID uint32 = 1
	// MinStartHeightDelay defines the minimal start height delay from the current blockchain height to kick off
	// sub-chain first block
	MinStartHeightDelay = 10
)

var (
	// MinSecurityDeposit represents the security deposit minimal required for start a sub-chain, which is 1M iotx
	// TODO: we should use IOTX unit instead
	MinSecurityDeposit = big.NewInt(0).Mul(big.NewInt(1000000000), big.NewInt(1 /*blockchain.Iotx*/))
)

// Protocol defines the protocol of handling sub-chain actions
type Protocol struct {
	chain     blockchain.Blockchain
	sf        state.Factory
	subChains map[uint32]*subChain
}

// NewProtocol instantiates the protocol of sub-chain
func NewProtocol(chain blockchain.Blockchain, sf state.Factory) *Protocol {
	return &Protocol{
		chain:     chain,
		sf:        sf,
		subChains: make(map[uint32]*subChain),
	}
}

// Handle handles how to mutate the state db given the sub-chain action
func (p *Protocol) Handle(act action.Action, ws state.WorkingSet) error {
	switch act.(type) {
	case *action.StartSubChain:
		return errors.Wrapf(
			p.handleStartSubChain(act.(*action.StartSubChain), ws),
			"error when handling start sub-chain action",
		)
	}

	// The action is not handled by this handler
	return nil
}

// Validate validates the sub-chain action
func (p *Protocol) Validate(act action.Action) error {
	switch act.(type) {
	case *action.StartSubChain:
		return errors.Wrapf(
			p.validateStartSubChain(act.(*action.StartSubChain), nil),
			"error when handling start sub-chain action",
		)
	}
	// The action is not validated by this handler
	return nil
}

func (p *Protocol) handleStartSubChain(start *action.StartSubChain, ws state.WorkingSet) error {
	if err := p.validateStartSubChain(start, ws); err != nil {
		return err
	}
	// TODO: persist this into state factory
	/*
		subChain := subChain{
			chainID:            start.ChainID(),
			securityDeposit:    start.SecurityDeposit(),
			operationDeposit:   start.OperationDeposit(),
			startHeight:        start.StartHeight(),
			parentHeightOffset: start.ParentHeightOffset(),
			ownerPublicKey:     start.OwnerPublicKey(),
			blocks:             make(map[uint64]*blockProof),
		}
	*/
	return nil
}

func (p *Protocol) validateStartSubChain(start *action.StartSubChain, ws state.WorkingSet) error {
	if start.ChainID() == MainChainID {
		return fmt.Errorf("%d is the chain ID reserved for main chain", start.ChainID())
	}
	if _, ok := p.subChains[start.ChainID()]; ok {
		return fmt.Errorf("%d is used by another sub-chain", start.ChainID())
	}
	var state *state.State
	var err error
	if ws == nil {
		state, err = p.sf.LoadOrCreateState(start.OwnerAddress(), 0)
	} else {
		state, err = ws.LoadOrCreateState(start.OwnerAddress(), 0)
	}
	if err != nil {
		return errors.Wrapf(err, "error when getting the state of address %s", start.OwnerAddress())
	}
	if start.SecurityDeposit().Cmp(MinSecurityDeposit) < 0 {
		return fmt.Errorf("security deposit is smaller than the minimal requirement %d", MinSecurityDeposit)
	}
	if state.Balance.Cmp(start.SecurityDeposit()) < 0 {
		return errors.New("sub-chain owner doesn't have enough balance for security deposit")
	}
	if state.Balance.Cmp(big.NewInt(0).Add(start.SecurityDeposit(), start.OperationDeposit())) < 0 {
		return errors.New("sub-chain owner doesn't have enough balance for operation deposit")
	}
	if start.StartHeight() < p.chain.TipHeight()+MinStartHeightDelay {
		return fmt.Errorf("sub-chain could be started no early than %d", p.chain.TipHeight()+MinStartHeightDelay)
	}
	return nil
}
