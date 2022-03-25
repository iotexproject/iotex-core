// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package gasstation

import (
	"context"
	"math/big"
	"sort"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
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
	bc        blockchain.Blockchain
	simulator SimulateFunc
	dao       BlockDAO
	cfg       config.GasStation
}

// NewGasStation creates a new gas station
func NewGasStation(bc blockchain.Blockchain, simulator SimulateFunc, dao BlockDAO, cfg config.GasStation) *GasStation {
	return &GasStation{
		bc:        bc,
		simulator: simulator,
		dao:       dao,
		cfg:       cfg,
	}
}

//isSystemAction determine whether input action belongs to system action
func (gs *GasStation) isSystemAction(act action.SealedEnvelope) bool {
	switch act.Action().(type) {
	case *action.GrantReward:
		return true
	case *action.PutPollResult:
		return true
	default:
		return false
	}
}

// SuggestGasPrice suggest gas price
func (gs *GasStation) SuggestGasPrice() (uint64, error) {
	var smallestPrices []*big.Int
	tip := gs.bc.TipHeight()

	endBlockHeight := uint64(0)
	if tip > uint64(gs.cfg.SuggestBlockWindow) {
		endBlockHeight = tip - uint64(gs.cfg.SuggestBlockWindow)
	}

	for height := tip; height > endBlockHeight; height-- {
		blk, err := gs.dao.GetBlockByHeight(height)
		if err != nil {
			return gs.cfg.DefaultGas, err
		}
		if len(blk.Actions) == 0 {
			continue
		}
		if len(blk.Actions) == 1 && gs.isSystemAction(blk.Actions[0]) {
			continue
		}
		smallestPrice := blk.Actions[0].GasPrice()
		for _, act := range blk.Actions {
			if gs.isSystemAction(act) {
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
		return gs.cfg.DefaultGas, nil
	}
	sort.Sort(bigIntArray(smallestPrices))
	gasPrice := smallestPrices[(len(smallestPrices)-1)*gs.cfg.Percentile/100].Uint64()
	if gasPrice < gs.cfg.DefaultGas {
		gasPrice = gs.cfg.DefaultGas
	}
	return gasPrice, nil
}

// EstimateGasForAction estimate gas for action
func (gs *GasStation) EstimateGasForAction(actPb *iotextypes.Action) (uint64, error) {
	selp, err := (&action.Deserializer{}).ActionToSealedEnvelope(actPb)
	if err != nil {
		return 0, err
	}
	// Special handling for executions
	if sc, ok := selp.Action().(*action.Execution); ok {
		callerAddr := selp.SrcPubkey().Address()
		if callerAddr == nil {
			return 0, errors.New("failed to get address")
		}
		ctx, err := gs.bc.Context(context.Background())
		if err != nil {
			return 0, err
		}
		_, receipt, err := gs.simulator(ctx, callerAddr, sc, gs.dao.GetBlockHash)
		if err != nil {
			return 0, err
		}
		return receipt.GasConsumed, nil
	}
	gas, err := selp.IntrinsicGas()
	if err != nil {
		return 0, err
	}
	return gas, nil
}

type bigIntArray []*big.Int

func (s bigIntArray) Len() int           { return len(s) }
func (s bigIntArray) Less(i, j int) bool { return s[i].Cmp(s[j]) < 0 }
func (s bigIntArray) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
