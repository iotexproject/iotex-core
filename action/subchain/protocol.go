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
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
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
	MinSecurityDeposit = big.NewInt(0).Mul(big.NewInt(1000000000), big.NewInt(blockchain.Iotx))
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
		if err := p.handleStartSubChain(act.(*action.StartSubChain), ws); err != nil {
			return errors.Wrapf(err, "error when handling start sub-chain action")
		}
	}
	// The action is not handled by this handler or no error
	return nil
}

// Validate validates the sub-chain action
func (p *Protocol) Validate(act action.Action) error {
	switch act.(type) {
	case *action.StartSubChain:
		if _, err := p.validateStartSubChain(act.(*action.StartSubChain), nil); err != nil {
			return errors.Wrapf(err, "error when handling start sub-chain action")
		}
	}
	// The action is not validated by this handler or no error
	return nil
}

func (p *Protocol) handleStartSubChain(start *action.StartSubChain, ws state.WorkingSet) error {
	account, err := p.validateStartSubChain(start, ws)
	if err != nil {
		return err
	}
	addr, err := createSubChainAddress(start.OwnerAddress(), start.Nonce())
	if err != nil {
		return err
	}
	sc := subChain{
		ChainID:            start.ChainID(),
		SecurityDeposit:    start.SecurityDeposit(),
		OperationDeposit:   start.OperationDeposit(),
		StartHeight:        start.StartHeight(),
		ParentHeightOffset: start.ParentHeightOffset(),
		OwnerPublicKey:     start.OwnerPublicKey(),
		CurrentHeight:      0,
	}
	if err := ws.PutState(addr, &sc); err != nil {
		return errors.Wrap(err, "error when putting sub-chain state")
	}
	account.Balance = big.NewInt(0).Sub(account.Balance, start.SecurityDeposit())
	account.Balance = big.NewInt(0).Sub(account.Balance, start.OperationDeposit())
	// TODO: this is not right, but currently the actions in a block is not processed according to the nonce
	if start.Nonce() > account.Nonce {
		account.Nonce = start.Nonce()
	}
	ownerPKHash, err := ownerAddressPKHash(start.OwnerAddress())
	if err != nil {
		return err
	}
	if err := ws.PutState(ownerPKHash, account); err != nil {
		return err
	}
	// TODO: update voting results because of owner account balance change
	return nil
}

func (p *Protocol) validateStartSubChain(start *action.StartSubChain, ws state.WorkingSet) (*state.Account, error) {
	if start.ChainID() == MainChainID {
		return nil, fmt.Errorf("%d is the chain ID reserved for main chain", start.ChainID())
	}
	if _, ok := p.subChains[start.ChainID()]; ok {
		return nil, fmt.Errorf("%d is used by another sub-chain", start.ChainID())
	}
	var account *state.Account
	var err error
	if ws == nil {
		account, err = p.sf.AccountState(start.OwnerAddress())
	} else {
		account, err = ws.CachedAccountState(start.OwnerAddress())
	}
	if err != nil {
		return nil, errors.Wrapf(err, "error when getting the state of address %s", start.OwnerAddress())
	}
	if start.SecurityDeposit().Cmp(MinSecurityDeposit) < 0 {
		return nil, fmt.Errorf("security deposit is smaller than the minimal requirement %d", MinSecurityDeposit)
	}
	if account.Balance.Cmp(start.SecurityDeposit()) < 0 {
		return nil, errors.New("sub-chain owner doesn't have enough balance for security deposit")
	}
	if account.Balance.Cmp(big.NewInt(0).Add(start.SecurityDeposit(), start.OperationDeposit())) < 0 {
		return nil, errors.New("sub-chain owner doesn't have enough balance for operation deposit")
	}
	if start.StartHeight() < p.chain.TipHeight()+MinStartHeightDelay {
		return nil, fmt.Errorf("sub-chain could be started no early than %d", p.chain.TipHeight()+MinStartHeightDelay)
	}
	return account, nil
}

func createSubChainAddress(ownerAddr string, nonce uint64) (hash.PKHash, error) {
	addr, err := address.IotxAddressToAddress(ownerAddr)
	if err != nil {
		return hash.ZeroPKHash, err
	}
	bytes := make([]byte, 8)
	enc.MachineEndian.PutUint64(bytes, nonce)
	return byteutil.BytesTo20B(hash.Hash160b(append(addr.Payload(), bytes...))), nil
}

func ownerAddressPKHash(ownerAddr string) (hash.PKHash, error) {
	addr, err := address.IotxAddressToAddress(ownerAddr)
	if err != nil {
		return hash.ZeroPKHash, errors.Wrapf(err, "cannot get the public key hash of address %s", ownerAddr)
	}
	return byteutil.BytesTo20B(addr.Payload()), nil
}
