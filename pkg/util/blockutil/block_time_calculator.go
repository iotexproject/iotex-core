package blockutil

import "time"

type (
	// BlockTimeCalculator calculates block time of a given height.
	BlockTimeCalculator struct {
		tipHeight        tipHeightFunc
		historyBlockTime blockTimeFunc
		blockInterval    time.Duration
	}

	tipHeightFunc func() uint64
	blockTimeFunc func(uint64) (time.Time, error)
)

// NewBlockTimeCalculator creates a new BlockTimeCalculator.
func NewBlockTimeCalculator(interval time.Duration, tipHeight tipHeightFunc, blockTime blockTimeFunc) *BlockTimeCalculator {
	return &BlockTimeCalculator{
		blockInterval:    interval,
		tipHeight:        tipHeight,
		historyBlockTime: blockTime,
	}
}

// CalculateBlockTime returns the block time of the given height.
// If the height is in the future, it will predict the block time according to the tip block time and interval.
// If the height is in the past, it will get the block time from indexer.
func (btc *BlockTimeCalculator) CalculateBlockTime(height uint64) (time.Time, error) {
	// get block time from indexer if height is in the past
	tipHeight := btc.tipHeight()
	if height <= tipHeight {
		return btc.historyBlockTime(height)
	}

	// predict block time according to tip block time and interval
	tipBlockTime, err := btc.historyBlockTime(tipHeight)
	if err != nil {
		return time.Time{}, err
	}
	return tipBlockTime.Add(time.Duration(height-tipHeight) * btc.blockInterval), nil
}
