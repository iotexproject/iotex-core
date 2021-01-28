// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package p2p

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/testingpb"

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
	b := func(_ context.Context, _ uint32, _ string, msg proto.Message) {
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
	u := func(_ context.Context, _ uint32, _ peer.AddrInfo, _ proto.Message) {}
	bootnodePort := testutil.RandomPort()
	bootnode := NewAgent(Network{Host: "127.0.0.1", Port: bootnodePort, ReconnectInterval: 150 * time.Second}, hash.ZeroHash256, b, u)
	require.NoError(t, bootnode.Start(ctx))
	require.NoError(t, testutil.WaitUntil(100*time.Millisecond, 10*time.Second, func() (b bool, e error) {
		ip := net.ParseIP("127.0.0.1")
		tcpAddr := net.TCPAddr{
			IP:   ip,
			Port: bootnodePort,
		}
		_, err := net.DialTCP("tcp", nil, &tcpAddr)
		return err == nil, nil
	}))
	addrs, err := bootnode.Self()
	require.NoError(t, err)
	for i := 0; i < n; i++ {
		port := bootnodePort + i + 1
		agent := NewAgent(Network{
			Host:              "127.0.0.1",
			Port:              port,
			BootstrapNodes:    []string{addrs[0].String()},
			ReconnectInterval: 150 * time.Second,
		}, hash.ZeroHash256, b, u)
		require.NoError(t, agent.Start(ctx))
		require.NoError(t, testutil.WaitUntil(100*time.Millisecond, 10*time.Second, func() (b bool, e error) {
			ip := net.ParseIP("127.0.0.1")
			tcpAddr := net.TCPAddr{
				IP:   ip,
				Port: port,
			}
			_, err := net.DialTCP("tcp", nil, &tcpAddr)
			return err == nil, nil
		}))
		agents = append(agents, agent)
	}

	for i := 0; i < n; i++ {
		require.NoError(t, agents[i].BroadcastOutbound(WitContext(ctx, Context{ChainID: 1}), &testingpb.TestPayload{
			MsgBody: []byte{uint8(i)},
		}))
		require.NoError(t, testutil.WaitUntil(100*time.Millisecond, 20*time.Second, func() (bool, error) {
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
	b := func(_ context.Context, _ uint32, _ string, _ proto.Message) {}
	u := func(_ context.Context, _ uint32, peer peer.AddrInfo, msg proto.Message) {
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

	bootnode := NewAgent(Network{Host: "127.0.0.1", Port: testutil.RandomPort(), ReconnectInterval: 150 * time.Second}, hash.ZeroHash256, b, u)
	require.NoError(t, bootnode.Start(ctx))

	addrs, err := bootnode.Self()
	require.NoError(t, err)
	for i := 0; i < n; i++ {
		agent := NewAgent(Network{
			Host:              "127.0.0.1",
			Port:              testutil.RandomPort(),
			BootstrapNodes:    []string{addrs[0].String()},
			ReconnectInterval: 150 * time.Second,
		}, hash.ZeroHash256, b, u)
		require.NoError(t, agent.Start(ctx))
		agents = append(agents, agent)
	}

	for i := 0; i < n; i++ {
		neighbors, err := agents[i].Neighbors(ctx)
		require.NoError(t, err)
		require.True(t, len(neighbors) > 0)
		for _, neighbor := range neighbors {
			require.NoError(t, agents[i].UnicastOutbound(WitContext(ctx, Context{ChainID: 1}), neighbor, &testingpb.TestPayload{
				MsgBody: []byte{uint8(i)},
			}))
		}
		require.NoError(t, testutil.WaitUntil(100*time.Millisecond, 20*time.Second, func() (bool, error) {
			mutex.RLock()
			defer mutex.RUnlock()
			info := agents[i].Info()
			return counts[uint8(i)] == len(neighbors) && src == info.ID.Pretty(), nil
		}))
	}
}
