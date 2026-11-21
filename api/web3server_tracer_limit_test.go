// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	. "github.com/agiledragon/gomonkey/v2"
	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

func TestStructLogLimit(t *testing.T) {
	for _, c := range []struct {
		cfg  *logger.Config
		want int
	}{
		{nil, defaultTraceOutputLimit},
		{&logger.Config{}, defaultTraceOutputLimit},
		{&logger.Config{Limit: -1}, defaultTraceOutputLimit},
		{&logger.Config{Limit: 4096}, 4096},
		{&logger.Config{Limit: maxTraceOutputLimit + 1}, maxTraceOutputLimit},
	} {
		require.Equal(t, c.want, structLogLimit(c.cfg))
	}
}

func TestParseTracerConfigLimit(t *testing.T) {
	options := gjson.Parse(`{"limit":4096}`)
	require.Equal(t, 4096, parseTracerConfig(&options).Config.Limit)
	options = gjson.Parse(`{}`)
	require.Zero(t, parseTracerConfig(&options).Config.Limit)
}

func TestNewStructLogTracerKeepsCallerConfig(t *testing.T) {
	// a block trace builds one tracer per transaction from the same config, so
	// applying the default limit must not write it back
	cfg := &logger.Config{}
	newStructLogTracer(cfg, func(error) {})
	require.Zero(t, cfg.Limit)
}

func TestAcquireTraceSlot(t *testing.T) {
	require := require.New(t)
	releases := make([]func(), 0, maxConcurrentTraces)
	for range maxConcurrentTraces {
		release, err := acquireTraceSlot()
		require.NoError(err)
		releases = append(releases, release)
	}
	_, err := acquireTraceSlot()
	require.ErrorIs(err, errTooManyTraces)

	releases[0]()
	release, err := acquireTraceSlot()
	require.NoError(err)
	release()
	for _, release := range releases[1:] {
		release()
	}
}

func TestTraceTxMaterializesResultWithinSlot(t *testing.T) {
	for _, resultErr := range []error{nil, errTraceOutputLimit} {
		t.Run(fmt.Sprint(resultErr), func(t *testing.T) {
			r := require.New(t)
			// Leave exactly one slot for traceTx. GetResult must still see all
			// slots occupied, and the returned tracer must use the cached result.
			for range maxConcurrentTraces - 1 {
				release, err := acquireTraceSlot()
				r.NoError(err)
				t.Cleanup(release)
			}
			calls := 0
			tracer := &tracers.Tracer{GetResult: func() (json.RawMessage, error) {
				calls++
				release, err := acquireTraceSlot()
				if err == nil {
					release()
				}
				r.ErrorIs(err, errTooManyTraces)
				return json.RawMessage(`{}`), resultErr
			}}
			patch := ApplyFunc(parseTracer, func(context.Context, *tracers.Context, *tracers.TraceConfig) (*tracers.Tracer, context.CancelFunc, error) {
				return tracer, func() {}, nil
			})
			defer patch.Reset()
			ctx := protocol.WithBlockchainCtx(context.Background(), protocol.BlockchainCtx{})
			core := &coreService{}
			_, _, result, err := core.traceTx(ctx, nil, nil, func(context.Context) ([]byte, *action.Receipt, error) {
				return nil, nil, nil
			})
			r.NoError(err)
			r.Equal(1, calls, "GetResult must run before traceTx returns")
			for range 2 {
				res, err := result.(*tracers.Tracer).GetResult()
				r.Equal(json.RawMessage(`{}`), res)
				r.ErrorIs(err, resultErr)
			}
			r.Equal(1, calls, "callers must not marshal the trace again")
			release, err := acquireTraceSlot()
			r.NoError(err, "the slot must be released even when GetResult fails")
			release()
		})
	}
}
