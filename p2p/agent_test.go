// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package p2p

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/protogen/testingpb"
	"github.com/iotexproject/iotex-core/testutil"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/stretchr/testify/require"
)

func TestBroadcast(t *testing.T) {
	for n := 1; n < 5; n++ {
		testBroadcastNumber(t, n, 10)
	}
}
func TestBroadcastLargeBlock(t *testing.T) {
	testBroadcastNumber(t, 4, 5*1024*1024)
}
func testBroadcastNumber(t *testing.T, n int, messagesize int) {
	ctx := context.Background()
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
	bh := func(name string) HandleBroadcastInbound {
		b := func(_ context.Context, _ uint32, msg proto.Message) {
			mutex.Lock()
			defer mutex.Unlock()
			testMsg, ok := msg.(*testingpb.TestPayload)
			require.True(t, ok)
			//t.Logf("%s receive message bodylen=%d", name, len(testMsg.MsgBody))
			idx := testMsg.MsgBody[0]
			if _, ok = counts[idx]; ok {
				counts[idx]++
			} else {
				counts[idx] = 1
			}
		}
		return b
	}

	u := func(_ context.Context, _ uint32, _ peerstore.PeerInfo, _ proto.Message) {}
	cfg := config.Config{
		Network: config.Network{Host: "127.0.0.1", Port: testutil.RandomPort()},
	}
	bootnode := NewAgent(cfg, bh("bootnode"), u)
	require.NoError(t, bootnode.Start(ctx))

	for i := 0; i < n; i++ {
		cfg := config.Config{
			Network: config.Network{
				Host:           "127.0.0.1",
				Port:           testutil.RandomPort(),
				BootstrapNodes: []string{bootnode.Self()[0].String()},
			},
		}
		name := fmt.Sprintf("agent%d", i)
		agent := NewAgent(cfg, bh(name), u)
		require.NoError(t, agent.Start(ctx))
		agents = append(agents, agent)
	}
	//log.L().Warn(fmt.Sprintf("all start complete...."))
	//must wait for heart beat for mesh link
	time.Sleep(pubsub.GossipSubHeartbeatInitialDelay)
	for i := 0; i < n; i++ {
		body := []byte{uint8(i)}
		body = append(body, make([]byte, messagesize-1)...)
		require.NoError(t, agents[i].BroadcastOutbound(WitContext(ctx, Context{ChainID: 1}), &testingpb.TestPayload{
			MsgBody: body,
		}))
		require.NoError(t, testutil.WaitUntil(100*time.Millisecond, 2*time.Second, func() (bool, error) {
			mutex.RLock()
			defer mutex.RUnlock()
			// Broadcast message will be skipped by the source node
			return counts[uint8(i)] == n, nil
		}))
	}
	//log.L().Warn(fmt.Sprintf("all broadcast complete..."))
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
			body := make([]byte, 5*1024*1024)
			body[0] = uint8(i)
			require.NoError(t, agents[i].UnicastOutbound(WitContext(ctx, Context{ChainID: 1}), neighbor, &testingpb.TestPayload{
				MsgBody: body,
			}))
		}
		require.NoError(t, testutil.WaitUntil(100*time.Millisecond, 10*time.Second, func() (bool, error) {
			mutex.RLock()
			defer mutex.RUnlock()
			return counts[uint8(i)] == n && src == agents[i].Info().ID.Pretty(), nil
		}))
	}
}
