// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package gasstation

import (
	"math/big"
	"sort"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

// GasStation provide gas related api
type GasStation struct {
	bc  blockchain.Blockchain
	cfg config.API
}

// NewGasStation creates a new gas station
func NewGasStation(bc blockchain.Blockchain, cfg config.API) *GasStation {
	return &GasStation{
		bc:  bc,
		cfg: cfg,
	}
}

// SuggestGasPrice suggest gas price
func (gs *GasStation) SuggestGasPrice() (uint64, error) {
	var smallestPrices []*big.Int
	tip := gs.bc.TipHeight()

	endBlockHeight := uint64(0)
	if tip > uint64(gs.cfg.GasStation.SuggestBlockWindow) {
		endBlockHeight = tip - uint64(gs.cfg.GasStation.SuggestBlockWindow)
	}

	for height := tip; height > endBlockHeight; height-- {
		blk, err := gs.bc.GetBlockByHeight(height)
		if err != nil {
			return gs.cfg.GasStation.DefaultGas, err
		}
		if len(blk.Actions) == 0 {
			continue
		}

		smallestPrice := blk.Actions[0].GasPrice()
		for _, action := range blk.Actions {
			if smallestPrice.Cmp(action.GasPrice()) == 1 {
				smallestPrice = action.GasPrice()
			}
		}
		smallestPrices = append(smallestPrices, smallestPrice)
	}

	if len(smallestPrices) == 0 {
		// return default price
		return gs.cfg.GasStation.DefaultGas, nil
	}
	sort.Sort(bigIntArray(smallestPrices))
	gasPrice := smallestPrices[(len(smallestPrices)-1)*gs.cfg.GasStation.Percentile/100].Uint64()
	if gasPrice < gs.cfg.GasStation.DefaultGas {
		gasPrice = gs.cfg.GasStation.DefaultGas
	}
	return gasPrice, nil
}

// EstimateGasForAction estimate gas for action
func (gs *GasStation) EstimateGasForAction(actPb *iotextypes.Action) (uint64, error) {
	var selp action.SealedEnvelope
	if err := selp.LoadProto(actPb); err != nil {
		return 0, err
	}
	// Special handling for executions
	if sc, ok := selp.Action().(*action.Execution); ok {
		callerAddr, err := address.FromBytes(selp.SrcPubkey().Hash())
		if err != nil {
			return 0, err
		}
		receipt, err := gs.bc.ExecuteContractRead(callerAddr, sc)
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
