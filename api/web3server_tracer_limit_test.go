// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package api

import (
	"testing"

	"github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
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
