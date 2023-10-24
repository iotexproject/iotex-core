package blockutil

import "time"

type (
	// BlockTimeCalculator calculates block time of a given height.
	BlockTimeCalculator struct {
		getBlockInterval    getBlockIntervalFn
		getTipHeight        getTipHeightFn
		getHistoryBlockTime getHistoryblockTimeFn
	}

	getBlockIntervalFn    func(uint64) time.Duration
	getTipHeightFn        func() uint64
	getHistoryblockTimeFn func(uint64) (time.Time, error)
)

// NewBlockTimeCalculator creates a new BlockTimeCalculator.
func NewBlockTimeCalculator(getBlockInterval getBlockIntervalFn, getTipHeight getTipHeightFn, getHistoryBlockTime getHistoryblockTimeFn) *BlockTimeCalculator {
	return &BlockTimeCalculator{
		getBlockInterval:    getBlockInterval,
		getTipHeight:        getTipHeight,
		getHistoryBlockTime: getHistoryBlockTime,
	}
}

// CalculateBlockTime returns the block time of the given height.
// If the height is in the future, it will predict the block time according to the tip block time and interval.
// If the height is in the past, it will get the block time from indexer.
func (btc *BlockTimeCalculator) CalculateBlockTime(height uint64) (time.Time, error) {
	// get block time from indexer if height is in the past
	tipHeight := btc.getTipHeight()
	if height <= tipHeight {
		return btc.getHistoryBlockTime(height)
	}

	// predict block time according to tip block time and interval
	tipBlockTime, err := btc.getHistoryBlockTime(tipHeight)
	if err != nil {
		return time.Time{}, err
	}
	return tipBlockTime.Add(time.Duration(height-tipHeight) * btc.getBlockInterval(height)), nil
}
