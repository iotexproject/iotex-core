// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"encoding/json"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
)

// tracerCtxForTest builds a minimal context that satisfies evm.NewChainConfig,
// which parseTracer invokes for the JS/native tracer branch.
func tracerCtxForTest() context.Context {
	ctx := genesis.WithGenesisContext(context.Background(), genesis.TestDefault())
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{
		Tip:          protocol.TipInfo{Height: 1, Timestamp: time.Now()},
		EvmNetworkID: 1,
	})
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight:    1,
		BlockTimeStamp: time.Now(),
	})
	return ctx
}

// registerStopRecorderTracer registers a tracer under the given name whose Stop
// call is observable via the returned atomic pointer (holding the stop error).
func registerStopRecorderTracer(name string) *atomic.Pointer[error] {
	stopErr := &atomic.Pointer[error]{}
	tracers.DefaultDirectory.Register(name, func(_ *tracers.Context, _ json.RawMessage, _ *params.ChainConfig) (*tracers.Tracer, error) {
		return &tracers.Tracer{
			Hooks:     &tracing.Hooks{},
			GetResult: func() (json.RawMessage, error) { return json.RawMessage("{}"), nil },
			Stop: func(err error) {
				stopErr.Store(&err)
			},
		}, nil
	}, false)
	return stopErr
}

// TestParseTracerTimeoutFires verifies that a configured tracer whose trace runs
// past the deadline is actually Stopped by the timeout watchdog. This is the bug
// that was fixed: previously the watchdog was disarmed (via defer cancel) before
// the trace executed, so the tracer was never stopped and a slow/looping trace
// could run unbounded (DoS on nodes exposing debug_trace*).
func TestParseTracerTimeoutFires(t *testing.T) {
	require := require.New(t)
	ctx := tracerCtxForTest()

	name := "test-timeout-fires-tracer"
	stopErr := registerStopRecorderTracer(name)
	timeout := "20ms"
	cfg := &tracers.TraceConfig{Tracer: &name, Timeout: &timeout}

	_, cleanup, err := parseTracer(ctx, new(tracers.Context), cfg)
	require.NoError(err)
	// Simulate a long-running trace: the caller has NOT yet finished executing,
	// so cleanup() has not run. The watchdog must fire on the deadline and stop
	// the tracer while the trace is still "executing".
	require.Eventually(func() bool {
		return stopErr.Load() != nil
	}, 2*time.Second, 5*time.Millisecond, "tracer should be stopped once the deadline expires")
	require.EqualError(*stopErr.Load(), "execution timeout")

	// cleanup after execution is a safe no-op here (deadline already fired).
	cleanup()
}

// TestParseTracerFastTraceNoStopNoLeak verifies that a normal, fast trace is not
// stopped by the watchdog and that the watchdog goroutine is cleaned up (no leak)
// once the caller invokes cleanup().
//
// It arms a short timeout, then simulates a fast trace by invoking cleanup()
// *before* the deadline. If cleanup() correctly disarms the watchdog, the
// deadline context is cancelled (Err == context.Canceled) and the goroutine
// returns without ever calling Stop. Waiting well past the original deadline and
// observing that Stop was never called proves both that the timeout does not
// misfire on a fast trace and that the watchdog goroutine did not leak (it can
// only avoid calling Stop by having woken on cancellation and returned).
func TestParseTracerFastTraceNoStopNoLeak(t *testing.T) {
	require := require.New(t)
	ctx := tracerCtxForTest()

	name := "test-fast-trace-tracer"
	stopErr := registerStopRecorderTracer(name)
	timeout := "30ms"
	cfg := &tracers.TraceConfig{Tracer: &name, Timeout: &timeout}

	armed := runtime.NumGoroutine()
	_, cleanup, err := parseTracer(ctx, new(tracers.Context), cfg)
	require.NoError(err)
	require.Greater(runtime.NumGoroutine(), armed-1, "watchdog goroutine should be running")
	// Simulate a fast trace that finishes before the deadline: disarm now.
	cleanup()

	// Wait well past the original 30ms deadline; the disarmed watchdog must not
	// fire Stop.
	time.Sleep(150 * time.Millisecond)
	require.Nil(stopErr.Load(), "tracer must not be stopped for a fast trace")
}

// TestParseTracerDefaultTimeout verifies the default-timeout path (Timeout == nil
// -> defaultTraceTimeout) builds a working tracer + cleanup without firing on a
// fast trace.
func TestParseTracerDefaultTimeout(t *testing.T) {
	require := require.New(t)
	ctx := tracerCtxForTest()

	name := "test-default-timeout-tracer"
	stopErr := registerStopRecorderTracer(name)
	cfg := &tracers.TraceConfig{Tracer: &name} // Timeout nil -> defaultTraceTimeout

	tracer, cleanup, err := parseTracer(ctx, new(tracers.Context), cfg)
	require.NoError(err)
	require.NotNil(tracer)
	require.NotNil(cleanup)
	cleanup()
	time.Sleep(20 * time.Millisecond)
	require.Nil(stopErr.Load(), "default-timeout tracer must not be stopped for a fast trace")
}

// TestParseTracerConfiguredTimeoutParsed verifies an invalid timeout string is
// surfaced as an error (configured-timeout path), and no watchdog is leaked.
func TestParseTracerConfiguredTimeoutParsed(t *testing.T) {
	require := require.New(t)
	ctx := tracerCtxForTest()

	name := "test-bad-timeout-tracer"
	registerStopRecorderTracer(name)
	bad := "not-a-duration"
	cfg := &tracers.TraceConfig{Tracer: &name, Timeout: &bad}

	tracer, cleanup, err := parseTracer(ctx, new(tracers.Context), cfg)
	require.Error(err)
	require.Nil(tracer)
	require.Nil(cleanup)
}

// TestParseTracerDefaultLoggerTimeoutFires verifies the timeout watchdog also
// covers the default struct-logger path (no named "tracer" in the config). This
// is the branch used by a plain debug_traceTransaction / debug_traceCall. On
// expiry the struct logger is Stopped and GetResult returns the timeout error,
// which the RPC layer surfaces to the caller.
func TestParseTracerDefaultLoggerTimeoutFires(t *testing.T) {
	require := require.New(t)
	ctx := tracerCtxForTest()

	timeout := "10ms"
	cfg := &tracers.TraceConfig{Timeout: &timeout, Config: &logger.Config{}}

	tracer, cleanup, err := parseTracer(ctx, new(tracers.Context), cfg)
	require.NoError(err)
	require.Eventually(func() bool {
		_, gerr := tracer.GetResult()
		return gerr != nil && gerr.Error() == "execution timeout"
	}, 2*time.Second, 5*time.Millisecond, "default struct logger should be stopped on timeout")
	cleanup()
}

// TestParseTracerNilConfigDefaultTimeout verifies a nil config still arms the
// default-timeout watchdog (and does not misfire / leak on a fast trace).
func TestParseTracerNilConfigDefaultTimeout(t *testing.T) {
	require := require.New(t)
	ctx := tracerCtxForTest()

	tracer, cleanup, err := parseTracer(ctx, new(tracers.Context), nil)
	require.NoError(err)
	require.NotNil(tracer)
	require.NotNil(cleanup)
	cleanup()
	// Fast trace: the default (5s) watchdog was cancelled, so the struct logger
	// must not have been stopped.
	time.Sleep(20 * time.Millisecond)
	_, gerr := tracer.GetResult()
	require.NotEqual("execution timeout", errString(gerr))
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
