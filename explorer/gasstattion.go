package explorer

import (
	"math/big"
	"sort"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
)

// GasStation provide gas related api
type GasStation struct {
	bc  blockchain.Blockchain
	cfg config.Explorer
}

// SuggestGasPrice suggest gas price
func (gs *GasStation) suggestGasPrice() (int64, error) {
	var smallestPrices []*big.Int
	tip := gs.bc.TipHeight()

	endBlockHeight := uint64(0)
	if tip > uint64(gs.cfg.GasStation.SuggestBlockWindow) {
		endBlockHeight = tip - uint64(gs.cfg.GasStation.SuggestBlockWindow)
	}

	for height := tip; height > endBlockHeight; height-- {
		blk, err := gs.bc.GetBlockByHeight(height)
		if err != nil {
			return int64(gs.cfg.GasStation.DefaultGas), err
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
		return int64(gs.cfg.GasStation.DefaultGas), nil
	}
	sort.Sort(bigIntArray(smallestPrices))
	gasPrice := smallestPrices[(len(smallestPrices)-1)*gs.cfg.GasStation.Percentile/100].Int64()
	if gasPrice < int64(gs.cfg.GasStation.DefaultGas) {
		gasPrice = int64(gs.cfg.GasStation.DefaultGas)
	}
	return gasPrice, nil
}

// EstimateGasForTransfer estimate gas for transfer
func (gs *GasStation) estimateGasForTransfer(tsfJSON explorer.SendTransferRequest) (int64, error) {
	actPb, err := convertExplorerTransferToActionPb(&tsfJSON, gs.cfg.MaxTransferPayloadBytes)
	if err != nil {
		return 0, err
	}
	tsf := &action.Transfer{}
	if err = tsf.LoadProto(actPb); err != nil {
		return 0, err
	}
	gas, err := tsf.IntrinsicGas()
	if err != nil {
		return 0, err
	}
	return int64(gas), nil
}

// EstimateGasForVote suggest gas for vote
func (gs *GasStation) estimateGasForVote() (int64, error) {
	v := &action.Vote{}
	gas, err := v.IntrinsicGas()
	if err != nil {
		return 0, err
	}
	return int64(gas), nil
}

// EstimateGasForSmartContract suggest gas for smart contract
func (gs *GasStation) estimateGasForSmartContract(execution explorer.Execution) (int64, error) {
	actPb, err := convertExplorerExecutionToActionPb(&execution)
	if err != nil {
		return 0, err
	}
	sc := &action.Execution{}
	if err := sc.LoadProto(actPb); err != nil {
		return 0, err
	}

	receipt, err := gs.bc.ExecuteContractRead(sc)
	if err != nil {
		return 0, err
	}
	return int64(receipt.GasConsumed), nil
}

type bigIntArray []*big.Int

func (s bigIntArray) Len() int           { return len(s) }
func (s bigIntArray) Less(i, j int) bool { return s[i].Cmp(s[j]) < 0 }
func (s bigIntArray) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
