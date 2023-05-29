package nodestats

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRPCLocalStats(t *testing.T) {
	require := require.New(t)
	stats := NewRPCLocalStats()
	stats.ReportCall(RpcReport{Method: "test", Success: true, HandlingTime: time.Duration(100 * time.Microsecond)}, 1)
	stats.ReportCall(RpcReport{Method: "test", Success: true, HandlingTime: time.Duration(200 * time.Microsecond)}, 2)
	stats.ReportCall(RpcReport{Method: "test", Success: true, HandlingTime: time.Duration(300 * time.Microsecond)}, 3)
	stats.ReportCall(RpcReport{Method: "test", Success: false, HandlingTime: time.Duration(400 * time.Microsecond)}, 4)
	val, ok := stats.allTimeStats.Load("test")
	require.True(ok)
	require.Equal(3, val.Successes)
	require.Equal(int64(200), val.AvgTimeOfSuccesses)
	require.Equal(int64(300), val.MaxTimeOfSuccess)
	require.Equal(int64(400), val.MaxTimeOfError)
	require.Equal(int64(400), val.AvgTimeOfErrors)
	require.Equal(1, val.Errors)
	require.Equal(int64(10), val.TotalSize)
	require.Equal(int64(2), val.AvgSize())
	stats.ReportCall(RpcReport{Method: "test2", Success: false, HandlingTime: time.Duration(400 * time.Microsecond)}, 400000)
	report := stats.BuildReport()
	t.Log(report)
}

func printa(a time.Time, n int, err error) {
	e := time.Since(a)
	fmt.Printf("e = %v, n=%d, err=%v\n", e, n, err)
}
func TestAA(t *testing.T) {
	var (
		n   int
		err error
	)
	defer func(start time.Time) { printa(start, n, err) }(time.Now())
	time.Sleep(1 * time.Second)
	n = 2
	err = fmt.Errorf("test")
}
