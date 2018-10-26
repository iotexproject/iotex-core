// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package subchain

import (
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/chainservice"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

const (
	// MainChainID reserves the ID for main chain
	MainChainID uint32 = 1
)

var (
	// MinSecurityDeposit represents the security deposit minimal required for start a sub-chain, which is 1M iotx
	MinSecurityDeposit = big.NewInt(0).Mul(big.NewInt(1000000000), big.NewInt(blockchain.Iotx))
	// subChainsInOperationKey is to find the used chain IDs in the state factory
	subChainsInOperationKey = byteutil.BytesTo20B(hash.Hash160b([]byte("subChainsInOperation")))
)

// Protocol defines the protocol of handling sub-chain actions
type Protocol struct {
	cfg              *config.Config
	p2p              network.Overlay
	dispatcher       dispatcher.Dispatcher
	rootChain        blockchain.Blockchain
	sf               state.Factory
	rootChainAPI     explorer.Explorer
	subChainServices map[uint32]*chainservice.ChainService
}

// NewProtocol instantiates the protocol of sub-chain
func NewProtocol(
	cfg *config.Config,
	p2p network.Overlay,
	dispatcher dispatcher.Dispatcher,
	rootChain blockchain.Blockchain,
	rootChainAPI explorer.Explorer,
) *Protocol {
	return &Protocol{
		cfg:              cfg,
		p2p:              p2p,
		dispatcher:       dispatcher,
		rootChain:        rootChain,
		sf:               rootChain.GetFactory(),
		rootChainAPI:     rootChainAPI,
		subChainServices: make(map[uint32]*chainservice.ChainService),
	}
}

// Handle handles how to mutate the state db given the sub-chain action
func (p *Protocol) Handle(act action.Action, ws state.WorkingSet) error {
	switch act := act.(type) {
	case *action.StartSubChain:
		if err := p.handleStartSubChain(act, ws); err != nil {
			return errors.Wrapf(err, "error when handling start sub-chain action")
		}
	case *action.PutBlock:
		if err := p.handlePutBlock(act, ws); err != nil {
			return errors.Wrapf(err, "error when handling start sub-chain action")
		}
	}
	// The action is not handled by this handler or no error
	return nil
}

// Validate validates the sub-chain action
func (p *Protocol) Validate(act action.Action) error {
	switch act := act.(type) {
	case *action.StartSubChain:
		if _, _, err := p.validateStartSubChain(act, nil); err != nil {
			return errors.Wrapf(err, "error when handling start sub-chain action")
		}
	}
	// The action is not validated by this handler or no error
	return nil
}
