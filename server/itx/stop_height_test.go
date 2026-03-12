// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package itx

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/pkg/probe"
)

func TestStopHeightController(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	controller := newStopHeightController(10)
	require.NotNil(controller)

	controller.markReached(9)
	select {
	case <-controller.doneCh():
		t.Fatal("controller should not stop below target height")
	default:
	}

	controller.markReached(10)
	select {
	case <-controller.doneCh():
	case <-time.After(time.Second):
		t.Fatal("controller did not stop at target height")
	}
	require.Equal(uint64(10), controller.reachedHeight())

	controller.markReached(12)
	require.Equal(uint64(10), controller.reachedHeight())
}

func TestStartServerStopAtHeight(t *testing.T) {
	require := require.New(t)
	cfg, cleanupPath := newConfig(t)
	defer cleanupPath()

	cfg.System.StopAtHeight = 1
	cfg.Genesis.BlockInterval = 100 * time.Millisecond

	svr, err := NewServer(cfg)
	require.NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	probeSvr := probe.New(cfg.System.HTTPStatsPort)
	require.NoError(probeSvr.Start(ctx))
	defer func() {
		require.NoError(probeSvr.Stop(context.Background()))
	}()

	done := make(chan struct{})
	go func() {
		StartServer(ctx, svr, probeSvr, cfg)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("server did not stop at configured height")
	}

	require.Equal(uint64(1), svr.rootChainService.Blockchain().TipHeight())
}
