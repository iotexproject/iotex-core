// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package gasstation

import (
	"context"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/params"
	"github.com/iotexproject/go-pkgs/cache"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

// BlockDAO represents the block data access object
type BlockDAO interface {
	GetBlockHash(uint64) (hash.Hash256, error)
	GetBlockByHeight(uint64) (*block.Block, error)
	GetReceipts(uint64) ([]*action.Receipt, error)
}

// SimulateFunc is function that simulate execution
type SimulateFunc func(context.Context, address.Address, *action.Execution, evm.GetBlockHash) ([]byte, *action.Receipt, error)

// GasStation provide gas related api
type GasStation struct {
	bc              blockchain.Blockchain
	dao             BlockDAO
	cfg             Config
	feeCache        cache.LRUCache
	percentileCache cache.LRUCache
}

// NewGasStation creates a new gas station
func NewGasStation(bc blockchain.Blockchain, dao BlockDAO, cfg Config) *GasStation {
	return &GasStation{
		bc:              bc,
		dao:             dao,
		cfg:             cfg,
		feeCache:        cache.NewThreadSafeLruCache(cfg.FeeHistoryCacheSize),
		percentileCache: cache.NewThreadSafeLruCache(cfg.FeeHistoryCacheSize),
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
		smallestPrice := blk.Actions[0].EffectiveGasPrice(blk.BaseFee())
		for _, receipt := range blk.Receipts {
			gasConsumed += receipt.GasConsumed
		}
		for _, act := range blk.Actions {
			if action.IsSystemAction(act) {
				continue
			}
			gasprice := act.EffectiveGasPrice(blk.BaseFee())
			if smallestPrice.Cmp(gasprice) == 1 {
				smallestPrice = gasprice
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

type blockFee struct {
	baseFee      *big.Int
	gasUsedRatio float64
	blobBaseFee  *big.Int
	blobGasRatio float64
}

type blockPercents struct {
	ascEffectivePriorityFees []*big.Int
}

// FeeHistory returns fee history over a series of blocks
func (gs *GasStation) FeeHistory(ctx context.Context, blocks, lastBlock uint64, rewardPercentiles []float64) (uint64, [][]*big.Int, []*big.Int, []float64, []*big.Int, []float64, error) {
	if blocks < 1 {
		return 0, nil, nil, nil, nil, nil, nil
	}
	maxFeeHistory := uint64(1024)
	if blocks > maxFeeHistory {
		log.T(ctx).Warn("Sanitizing fee history length", zap.Uint64("requested", blocks), zap.Uint64("truncated", maxFeeHistory))
		blocks = maxFeeHistory
	}
	if blocks > lastBlock {
		blocks = lastBlock
	}
	for i, p := range rewardPercentiles {
		if p < 0 || p > 100 {
			return 0, nil, nil, nil, nil, nil, status.Error(codes.InvalidArgument, "percentile must be in [0, 100]")
		}
		if i > 0 && p <= rewardPercentiles[i-1] {
			return 0, nil, nil, nil, nil, nil, status.Error(codes.InvalidArgument, "percentiles must be in ascending order")
		}
	}

	var (
		rewards           = make([][]*big.Int, 0, blocks)
		baseFees          = make([]*big.Int, blocks+1)
		gasUsedRatios     = make([]float64, blocks)
		blobBaseFees      = make([]*big.Int, blocks+1)
		blobGasUsedRatios = make([]float64, blocks)
		g                 = gs.bc.Genesis()
		lastBlk           *block.Block
	)
	for i := uint64(0); i < blocks; i++ {
		height := lastBlock - i
		if blkFee, ok := gs.feeCache.Get(height); ok {
			// cache hit
			log.T(ctx).Debug("fee cache hit", zap.Uint64("height", height))
			bf := blkFee.(*blockFee)
			baseFees[i] = bf.baseFee
			gasUsedRatios[i] = bf.gasUsedRatio
			blobBaseFees[i] = bf.blobBaseFee
			blobGasUsedRatios[i] = bf.blobGasRatio
		} else {
			// read block fee from dao
			log.T(ctx).Debug("fee cache miss", zap.Uint64("height", height))
			blk, err := gs.dao.GetBlockByHeight(height)
			if err != nil {
				return 0, nil, nil, nil, nil, nil, status.Error(codes.NotFound, err.Error())
			}
			if i == 0 {
				lastBlk = blk
			}
			baseFees[i] = blk.BaseFee()
			gasUsedRatios[i] = float64(blk.GasUsed()) / float64(g.BlockGasLimitByHeight(blk.Height()))
			blobBaseFees[i] = protocol.CalcBlobFee(blk.ExcessBlobGas())
			blobGasUsedRatios[i] = float64(blk.BlobGasUsed()) / float64(params.MaxBlobGasPerBlock)
			gs.feeCache.Add(height, &blockFee{
				baseFee:      baseFees[i],
				gasUsedRatio: gasUsedRatios[i],
				blobBaseFee:  blobBaseFees[i],
				blobGasRatio: blobGasUsedRatios[i],
			})
		}
		// block priority fee percentiles
		if len(rewardPercentiles) > 0 {
			if blkPercents, ok := gs.percentileCache.Get(height); ok {
				log.T(ctx).Debug("percentile cache hit", zap.Uint64("height", height))
				rewards = append(rewards, feesPercentiles(blkPercents.(*blockPercents).ascEffectivePriorityFees, rewardPercentiles))
			} else {
				log.T(ctx).Debug("percentile cache miss", zap.Uint64("height", height))
				receipts, err := gs.dao.GetReceipts(height)
				if err != nil {
					return 0, nil, nil, nil, nil, nil, status.Error(codes.NotFound, err.Error())
				}
				fees := make([]*big.Int, 0, len(receipts))
				for _, r := range receipts {
					if pf := r.PriorityFee(); pf != nil {
						fees = append(fees, pf)
					} else {
						fees = append(fees, big.NewInt(0))
					}
				}
				sort.Slice(fees, func(i, j int) bool {
					return fees[i].Cmp(fees[j]) < 0
				})
				rewards = append(rewards, feesPercentiles(fees, rewardPercentiles))
				gs.percentileCache.Add(height, &blockPercents{
					ascEffectivePriorityFees: fees,
				})
			}
		}
	}
	// fill next block base fee
	if lastBlk == nil {
		blk, err := gs.dao.GetBlockByHeight(lastBlock)
		if err != nil {
			return 0, nil, nil, nil, nil, nil, status.Error(codes.NotFound, err.Error())
		}
		lastBlk = blk
	}
	baseFees[blocks] = protocol.CalcBaseFee(g.Blockchain, &protocol.TipInfo{
		Height:  lastBlock,
		GasUsed: lastBlk.GasUsed(),
		BaseFee: lastBlk.BaseFee(),
	})
	blobBaseFees[blocks] = protocol.CalcBlobFee(protocol.CalcExcessBlobGas(lastBlk.ExcessBlobGas(), lastBlk.BlobGasUsed()))
	return lastBlock - blocks + 1, rewards, baseFees, gasUsedRatios, blobBaseFees, blobGasUsedRatios, nil
}

func feesPercentiles(ascFees []*big.Int, percentiles []float64) []*big.Int {
	res := make([]*big.Int, len(percentiles))
	for i, p := range percentiles {
		idx := int(float64(len(ascFees)) * p)
		if idx >= len(ascFees) {
			idx = len(ascFees) - 1
		}
		res[i] = ascFees[idx]
	}
	return res
}
