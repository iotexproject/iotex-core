package nodestats

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRPCLocalStats(t *testing.T) {
	require := require.New(t)
	stats := NewAPILocalStats()
	stats.ReportCall(APIReport{Method: "test", Success: true, HandlingTime: time.Duration(100 * time.Microsecond)}, 1)
	stats.ReportCall(APIReport{Method: "test", Success: true, HandlingTime: time.Duration(200 * time.Microsecond)}, 2)
	stats.ReportCall(APIReport{Method: "test", Success: true, HandlingTime: time.Duration(300 * time.Microsecond)}, 3)
	stats.ReportCall(APIReport{Method: "test", Success: false, HandlingTime: time.Duration(400 * time.Microsecond)}, 4)
	v, ok := stats.allTimeStats.Load("test")
	require.True(ok)
	val := v.(*apiMethodStats)
	require.Equal(3, val.Successes)
	require.Equal(int64(200), val.AvgTimeOfSuccesses)
	require.Equal(int64(300), val.MaxTimeOfSuccess)
	require.Equal(int64(400), val.MaxTimeOfError)
	require.Equal(int64(400), val.AvgTimeOfErrors)
	require.Equal(1, val.Errors)
	require.Equal(int64(10), val.TotalSize)
	require.Equal(int64(2), val.AvgSize())
	stats.ReportCall(APIReport{Method: "test2", Success: false, HandlingTime: time.Duration(400 * time.Microsecond)}, 400000)
	report := stats.BuildReport()
	t.Log(report)
}
