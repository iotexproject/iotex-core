// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package itx

import (
	"context"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/probe"
	"github.com/iotexproject/iotex-core/test/identityset"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
)

func TestNewServer(t *testing.T) {
	require := require.New(t)
	s, err := NewServer(config.Default)
	require.NoError(err)
	require.NotNil(s)

	cfg := config.Default
	cfg.Consensus.Scheme = config.RollDPoSScheme
	cfg.Genesis.EnableGravityChainVoting = true
	cfg.System.HeartbeatInterval = 10
	cfg.System.HTTPAdminPort = 1000
	ss, err := NewServer(config.Default)
	require.NoError(err)
	require.NotNil(ss)
}
func TestNewInMemTestServer(t *testing.T) {
	require := require.New(t)
	s, err := NewInMemTestServer(config.Default)
	require.NoError(err)
	require.NotNil(s)
}
func TestStartStop(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	cfg := config.Default
	cfg.Consensus.Scheme = config.RollDPoSScheme
	s, err := NewServer(cfg)
	require.NoError(err)
	require.NotNil(s)
	err = s.Start(ctx)
	require.NoError(err)
	err = s.Stop(ctx)
	require.NoError(err)
	err = s.NewSubChainService(cfg)
	require.NoError(err)
	err = s.StopChainService(ctx, 0)
	require.Error(err)

	ss, err := NewServer(cfg)
	require.NoError(err)
	require.NotNil(ss)
	err = ss.StopChainService(ctx, 1)
	require.NoError(err)
}
func TestP2PAgent(t *testing.T) {
	require := require.New(t)
	s, err := NewServer(config.Default)
	require.NoError(err)
	require.NotNil(s)
	agent := s.P2PAgent()
	require.NotNil(agent)

	cs := s.ChainService(1)
	require.NotNil(cs)

	ds := s.Dispatcher()
	require.NotNil(ds)
}
func TestStartServer(t *testing.T) {
	require := require.New(t)
	cfg := config.Default
	s, err := NewServer(cfg)
	cfg.Consensus.Scheme = config.RollDPoSScheme
	cfg.Genesis.EnableGravityChainVoting = true
	require.NoError(err)
	require.NotNil(s)
	require.Panics(func() { StartServer(context.Background(), s, nil, cfg) }, "Probe server is nil")

	cfg, err = config.New()
	require.NoError(err)
	cfg.Consensus.Scheme = config.RollDPoSScheme
	cfg.Genesis.EnableGravityChainVoting = true
	ss, err := NewServer(cfg)
	require.NoError(err)
	require.NotNil(ss)
	ctx, cancel := context.WithCancel(context.Background())
	livenessCtx, livenessCancel := context.WithCancel(context.Background())
	probeSvr := probe.New(cfg.System.HTTPStatsPort)
	err = probeSvr.Start(ctx)
	require.NoError(err)
	go StartServer(ctx, ss, probeSvr, cfg)
	time.Sleep(time.Second * 2)

	rap := block.RunnableActionsBuilder{}
	ra := rap.
		SetHeight(1).
		SetTimeStamp(time.Now()).
		Build(identityset.PrivateKey(0).PublicKey())
	blk, err := block.NewBuilder(ra).
		SetVersion(1).
		SetReceiptRoot(hash.Hash256b([]byte("hello, world!"))).
		SetDeltaStateDigest(hash.Hash256b([]byte("world, hello!"))).
		SetPrevBlockHash(hash.Hash256b([]byte("hello, block!"))).
		SignAndBuild(identityset.PrivateKey(0))
	require.NoError(err)

	go func() {
		err = ss.HandleBlock(&blk)
		require.NoError(err)
	}()

	time.Sleep(time.Second * 2)
	cancel()
	err = probeSvr.Stop(livenessCtx)
	require.NoError(err)
	livenessCancel()
}
