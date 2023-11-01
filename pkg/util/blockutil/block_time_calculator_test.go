package blockutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBlockTimeCalculator_CalculateBlockTime(t *testing.T) {
	r := require.New(t)
	interval := 5 * time.Second
	intervalFn := func(h uint64) time.Duration {
		return 5 * time.Second
	}
	tipHeight := uint64(100)
	tipHeightF := func() uint64 { return tipHeight }
	baseTime, err := time.Parse("2006-01-02T15:04:05.000Z", "2022-01-01T00:00:00.000Z")
	r.NoError(err)
	historyBlockTimeF := func(height uint64) (time.Time, error) { return baseTime.Add(time.Hour * time.Duration(height)), nil }
	btc, err := NewBlockTimeCalculator(intervalFn, tipHeightF, historyBlockTimeF)
	r.NoError(err)

	historyWrapper := func(height uint64) time.Time {
		t, err := historyBlockTimeF(height)
		r.NoError(err)
		return t
	}
	cases := []struct {
		name   string
		height uint64
		want   time.Time
		errMsg string
	}{
		{"height is in the past", tipHeight - 1, historyWrapper(tipHeight - 1), ""},
		{"height is in the past I", tipHeight, historyWrapper(tipHeight), ""},
		{"height is in the future", tipHeight + 1, historyWrapper(tipHeight).Add(interval), ""},
		{"height is in the future I", tipHeight + 2, historyWrapper(tipHeight).Add(2 * interval), ""},
		{"height is not overflow", tipHeight + (1<<63-1)/uint64(interval), historyWrapper(tipHeight).Add((1<<63 - 1) / interval * interval), ""},
		{"height is overflow", tipHeight + (1<<63-1)/uint64(interval) + 1, time.Time{}, "height overflow"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := btc.CalculateBlockTime(c.height)
			if c.errMsg != "" {
				r.ErrorContains(err, c.errMsg)
				return
			}
			r.NoError(err)
			r.Equal(c.want, got)
		})
	}
}
