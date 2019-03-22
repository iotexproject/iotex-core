// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package p2p

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/protogen/testingpb"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestBroadcast(t *testing.T) {
	ctx := context.Background()
	n := 10
	agents := make([]*Agent, 0)
	defer func() {
		var err error
		for _, agent := range agents {
			err = agent.Stop(ctx)
		}
		require.NoError(t, err)
	}()
	counts := make(map[uint8]int)
	var mutex sync.RWMutex
	b := func(_ context.Context, _ uint32, msg proto.Message) {
		mutex.Lock()
		defer mutex.Unlock()
		testMsg, ok := msg.(*testingpb.TestPayload)
		require.True(t, ok)
		idx := testMsg.MsgBody[0]
		if _, ok = counts[idx]; ok {
			counts[idx]++
		} else {
			counts[idx] = 1
		}
	}
	u := func(_ context.Context, _ uint32, _ peerstore.PeerInfo, _ proto.Message) {}
	cfg := config.Config{
		Network: config.Network{Host: "127.0.0.1", Port: testutil.RandomPort()},
	}
	bootnode := NewAgent(cfg, b, u)
	require.NoError(t, bootnode.Start(ctx))

	for i := 0; i < n; i++ {
		cfg := config.Config{
			Network: config.Network{
				Host:           "127.0.0.1",
				Port:           testutil.RandomPort(),
				BootstrapNodes: []string{bootnode.Self()[0].String()},
			},
		}
		agent := NewAgent(cfg, b, u)
		require.NoError(t, agent.Start(ctx))
		agents = append(agents, agent)
	}

	for i := 0; i < n; i++ {
		require.NoError(t, agents[i].BroadcastOutbound(WitContext(ctx, Context{ChainID: 1}), &testingpb.TestPayload{
			MsgBody: []byte{uint8(i)},
		}))
		require.NoError(t, testutil.WaitUntil(100*time.Millisecond, 10*time.Second, func() (bool, error) {
			mutex.RLock()
			defer mutex.RUnlock()
			// Broadcast message will be skipped by the source node
			return counts[uint8(i)] == n, nil
		}))
	}
}

func TestUnicast(t *testing.T) {
	ctx := context.Background()
	n := 10
	agents := make([]*Agent, 0)
	defer func() {
		var err error
		for _, agent := range agents {
			err = agent.Stop(ctx)
		}
		require.NoError(t, err)
	}()
	counts := make(map[uint8]int)
	var src string
	var mutex sync.RWMutex
	b := func(_ context.Context, _ uint32, _ proto.Message) {}
	u := func(_ context.Context, _ uint32, peer peerstore.PeerInfo, msg proto.Message) {
		mutex.Lock()
		defer mutex.Unlock()
		testMsg, ok := msg.(*testingpb.TestPayload)
		require.True(t, ok)
		idx := testMsg.MsgBody[0]
		if _, ok = counts[idx]; ok {
			counts[idx]++
		} else {
			counts[idx] = 1
		}
		src = peer.ID.Pretty()
	}

	bootnode := NewAgent(config.Config{
		Network: config.Network{Host: "127.0.0.1", Port: testutil.RandomPort()},
	}, b, u)
	require.NoError(t, bootnode.Start(ctx))

	for i := 0; i < n; i++ {
		cfg := config.Config{
			Network: config.Network{
				Host:           "127.0.0.1",
				Port:           testutil.RandomPort(),
				BootstrapNodes: []string{bootnode.Self()[0].String()},
			},
		}
		agent := NewAgent(cfg, b, u)
		require.NoError(t, agent.Start(ctx))
		agents = append(agents, agent)
	}

	for i := 0; i < n; i++ {
		neighbors, err := agents[i].Neighbors(ctx)
		require.NoError(t, err)
		require.Equal(t, n, len(neighbors))
		for _, neighbor := range neighbors {
			require.NoError(t, agents[i].UnicastOutbound(WitContext(ctx, Context{ChainID: 1}), neighbor, &testingpb.TestPayload{
				MsgBody: []byte{uint8(i)},
			}))
		}
		require.NoError(t, testutil.WaitUntil(100*time.Millisecond, 10*time.Second, func() (bool, error) {
			mutex.RLock()
			defer mutex.RUnlock()
			return counts[uint8(i)] == n && src == agents[i].Info().ID.Pretty(), nil
		}))
	}
}
