package testutil

import (
	"time"

	"github.com/iotexproject/iotex-core/v2/pkg/util/blockutil"
)

func DummyBlockTimeBuilder() *blockutil.BlockTimeCalculatorBuilder {
	return blockutil.NewBlockTimeCalculatorBuilder().
		SetBlockInterval(func(uint64) time.Duration { return 5 * time.Second }).
		SetTipHeight(func() uint64 { return 1 }).
		SetHistoryBlockTime(func(uint64) (time.Time, error) { return time.Now(), nil })
}
