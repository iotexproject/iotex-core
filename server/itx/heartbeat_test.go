// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package itx

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/probe"
)

func TestNewHeartbeatHandler(t *testing.T) {
	require := require.New(t)
	cfg := config.Default
	s, err := NewServer(cfg)
	cfg.Consensus.Scheme = config.RollDPoSScheme
	cfg.Genesis.EnableGravityChainVoting = true
	require.NoError(err)
	require.NotNil(s)
	handler := NewHeartbeatHandler(s)
	require.NotNil(handler)
	require.Panics(func() { handler.Log() }, "P2pAgent is nil")

	ctx, cancel := context.WithCancel(context.Background())
	livenessCtx, livenessCancel := context.WithCancel(context.Background())
	probeSvr := probe.New(cfg.System.HTTPStatsPort)
	err = probeSvr.Start(ctx)
	require.NoError(err)
	go StartServer(ctx, s, probeSvr, cfg)
	time.Sleep(time.Second * 2)
	handler.Log()
	cancel()
	err = probeSvr.Stop(livenessCtx)
	require.NoError(err)
	livenessCancel()
}
