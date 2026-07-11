package api

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/testutil"
)

// TestGRPCServerStopBeforeServe reproduces the flaky-CI crash where Stop()
// wins the race against the Serve goroutine spawned by Start(): Serve then
// returns grpc.ErrServerStopped, which must be treated as a clean shutdown
// rather than a fatal error (log.Fatal exits the whole process, killing the
// test binary and any embedding node).
func TestGRPCServerStopBeforeServe(t *testing.T) {
	r := require.New(t)
	for i := 0; i < 20; i++ {
		svr := NewGRPCServer(nil, nil, testutil.RandomPort(), 100)
		r.NoError(svr.Start(context.Background()))
		// stop immediately so Stop frequently beats the Serve goroutine
		r.NoError(svr.Stop(context.Background()))
	}
	// give the Serve goroutines time to observe the stopped server; with the
	// bug this Fatals and kills the test process
	time.Sleep(300 * time.Millisecond)
}
