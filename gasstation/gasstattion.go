// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package gasstation

import (
	"context"
	"math/big"
	"sort"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
)

// BlockDAO represents the block data access object
type BlockDAO interface {
	GetBlockHash(uint64) (hash.Hash256, error)
	GetBlockByHeight(uint64) (*block.Block, error)
}

// SimulateFunc is function that simulate execution
type SimulateFunc func(context.Context, address.Address, *action.Execution, evm.GetBlockHash) ([]byte, *action.Receipt, error)

// GasStation provide gas related api
type GasStation struct {
	bc  blockchain.Blockchain
	dao BlockDAO
	cfg Config
}

// NewGasStation creates a new gas station
func NewGasStation(bc blockchain.Blockchain, dao BlockDAO, cfg Config) *GasStation {
	return &GasStation{
		bc:  bc,
		dao: dao,
		cfg: cfg,
	}
}

// SuggestGasPrice suggest gas price
func (gs *GasStation) SuggestGasPrice() (uint64, error) {
	var (
		smallestPrices []*big.Int
		endBlockHeight uint64
		tip            = gs.bc.TipHeight()
		g              = gs.bc.Genesis()
	)
	if tip > uint64(gs.cfg.SuggestBlockWindow) {
		endBlockHeight = tip - uint64(gs.cfg.SuggestBlockWindow)
	}
	maxGas := g.BlockGasLimitByHeight(tip) * (tip - endBlockHeight)
	defaultGasPrice := gs.cfg.DefaultGas
	gasConsumed := uint64(0)
	for height := tip; height > endBlockHeight; height-- {
		blk, err := gs.dao.GetBlockByHeight(height)
		if err != nil {
			return defaultGasPrice, err
		}
		if len(blk.Actions) == 0 {
			continue
		}
		if len(blk.Actions) == 1 && action.IsSystemAction(blk.Actions[0]) {
			continue
		}
		smallestPrice := blk.Actions[0].GasPrice()
		for _, receipt := range blk.Receipts {
			gasConsumed += receipt.GasConsumed
		}
		for _, act := range blk.Actions {
			if action.IsSystemAction(act) {
				continue
			}
			if smallestPrice.Cmp(act.GasPrice()) == 1 {
				smallestPrice = act.GasPrice()
			}
		}
		smallestPrices = append(smallestPrices, smallestPrice)
	}
	if len(smallestPrices) == 0 {
		// return default price
		return defaultGasPrice, nil
	}
	sort.Slice(smallestPrices, func(i, j int) bool {
		return smallestPrices[i].Cmp(smallestPrices[j]) < 0
	})
	gasPrice := smallestPrices[(len(smallestPrices)-1)*gs.cfg.Percentile/100].Uint64()
	switch {
	case gasConsumed > maxGas/2:
		gasPrice += gasPrice / 10
	case gasConsumed < maxGas/5:
		gasPrice -= gasPrice / 10
	}
	if gasPrice < defaultGasPrice {
		gasPrice = defaultGasPrice
	}
	return gasPrice, nil
}
