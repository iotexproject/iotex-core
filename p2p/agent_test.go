// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package p2p

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-p2p"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/testingpb"

	"github.com/iotexproject/iotex-core/testutil"
)

func TestDummyAgent(t *testing.T) {
	require := require.New(t)
	a := NewDummyAgent()
	require.NoError(a.Start(nil))
	require.NoError(a.Stop(nil))
	require.NoError(a.BroadcastOutbound(nil, nil))
	require.NoError(a.UnicastOutbound(nil, peer.AddrInfo{}, nil))
	info, err := a.Info()
	require.Equal(peer.AddrInfo{}, info)
	require.NoError(err)
	addrs, err := a.Self()
	require.Nil(addrs)
	require.NoError(err)
	neighbors, err := a.ConnectedPeers()
	require.Nil(neighbors)
	require.NoError(err)
}

func TestBroadcast(t *testing.T) {
	r := require.New(t)

	ctx := context.Background()
	n := 10
	agents := make([]Agent, 0)
	defer func() {
		var err error
		for _, agent := range agents {
			err = agent.Stop(ctx)
		}
		r.NoError(err)
	}()
	counts := make(map[uint8]int)
	var mutex sync.RWMutex
	b := func(_ context.Context, _ uint32, _ string, msg proto.Message) {
		mutex.Lock()
		defer mutex.Unlock()
		testMsg, ok := msg.(*testingpb.TestPayload)
		r.True(ok)
		idx := testMsg.MsgBody[0]
		if _, ok = counts[idx]; ok {
			counts[idx]++
		} else {
			counts[idx] = 1
		}
	}
	u := func(_ context.Context, _ uint32, _ peer.AddrInfo, _ proto.Message) {}

	bootnodePort := testutil.RandomPort()
	bootnode, err := p2p.NewHost(context.Background(), p2p.DHTProtocolID(1), p2p.Port(bootnodePort), p2p.SecureIO(), p2p.MasterKey("bootnode"))
	r.NoError(err)
	bootnodeAddr := bootnode.Addresses()

	for i := 0; i < n; i++ {
		port := bootnodePort + i + 1
		agent := NewAgent(Config{
			Host:              "127.0.0.1",
			Port:              port,
			BootstrapNodes:    []string{bootnodeAddr[0].String()},
			ReconnectInterval: 150 * time.Second,
		}, 1, hash.ZeroHash256, b, u)
		agent.Start(ctx)
		agents = append(agents, agent)
	}

	for i := 0; i < n; i++ {
		r.NoError(agents[i].BroadcastOutbound(ctx, &testingpb.TestPayload{
			MsgBody: []byte{uint8(i)},
		}))
		r.NoError(testutil.WaitUntil(100*time.Millisecond, 20*time.Second, func() (bool, error) {
			mutex.RLock()
			defer mutex.RUnlock()
			// Broadcast message will be skipped by the source node
			return counts[uint8(i)] == n-1, nil
		}))
	}
}

func TestUnicast(t *testing.T) {
	r := require.New(t)

	ctx := context.Background()
	n := 10
	agents := make([]Agent, 0)
	defer func() {
		var err error
		for _, agent := range agents {
			err = agent.Stop(ctx)
		}
		r.NoError(err)
	}()
	counts := make(map[uint8]int)
	var src string
	var mutex sync.RWMutex
	b := func(_ context.Context, _ uint32, _ string, _ proto.Message) {}
	u := func(_ context.Context, _ uint32, peer peer.AddrInfo, msg proto.Message) {
		mutex.Lock()
		defer mutex.Unlock()
		testMsg, ok := msg.(*testingpb.TestPayload)
		r.True(ok)
		idx := testMsg.MsgBody[0]
		if _, ok = counts[idx]; ok {
			counts[idx]++
		} else {
			counts[idx] = 1
		}
		src = peer.ID.Pretty()
	}

	bootnode, err := p2p.NewHost(context.Background(), p2p.DHTProtocolID(2), p2p.Port(testutil.RandomPort()), p2p.SecureIO(), p2p.MasterKey("bootnode"))
	r.NoError(err)
	addrs := bootnode.Addresses()
	for i := 0; i < n; i++ {
		agent := NewAgent(Config{
			Host:              "127.0.0.1",
			Port:              testutil.RandomPort(),
			BootstrapNodes:    []string{addrs[0].String()},
			ReconnectInterval: 150 * time.Second,
			MasterKey:         strconv.Itoa(i),
		}, 2, hash.ZeroHash256, b, u)
		r.NoError(agent.Start(ctx))
		agents = append(agents, agent)
	}

	for i := 0; i < n; i++ {
		neighbors, err := agents[i].ConnectedPeers()
		r.NoError(err)
		r.True(len(neighbors) >= n/3)
		for _, neighbor := range neighbors {
			r.NoError(agents[i].UnicastOutbound(ctx, neighbor, &testingpb.TestPayload{
				MsgBody: []byte{uint8(i)},
			}))
		}
		r.NoError(testutil.WaitUntil(100*time.Millisecond, 20*time.Second, func() (bool, error) {
			mutex.RLock()
			defer mutex.RUnlock()
			info, err := agents[i].Info()
			r.NoError(err)
			return counts[uint8(i)] == len(neighbors) && src == info.ID.Pretty(), nil
		}))
	}
}
