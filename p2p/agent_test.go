// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package p2p

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-p2p"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/testingpb"

	"github.com/iotexproject/iotex-core/v2/testutil"
)

const (
	_blockNetwork     = ""
	_consensusNetwork = "consensus"
	_actionNetwork    = "action"
)

func TestDummyAgent(t *testing.T) {
	require := require.New(t)
	a := NewDummyAgent().Subnet(_blockNetwork)
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
			MaxMessageSize:    p2p.DefaultConfig.MaxMessageSize,
		}, 1, hash.ZeroHash256, b, u, JoinSubnet(_blockNetwork))
		r.NoError(agent.Start(ctx))
		agents = append(agents, agent)
	}

	for i := 0; i < n; i++ {
		r.NoError(agents[i].Subnet(_blockNetwork).BroadcastOutbound(ctx, &testingpb.TestPayload{
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

func TestNetworkSeparation(t *testing.T) {
	require := require.New(t)

	ctx := context.Background()
	n := 10
	agents := make([]Agent, 0)
	defer func() {
		var err error
		for _, agent := range agents {
			err = agent.Stop(ctx)
		}
		require.NoError(err)
	}()
	counts := make(map[uint8]int)
	var mutex sync.RWMutex
	b := func(idx uint8) HandleBroadcastInbound {
		return func(_ context.Context, _ uint32, _ string, msg proto.Message) {
			mutex.Lock()
			defer mutex.Unlock()
			_, ok := msg.(*testingpb.TestPayload)
			require.True(ok)
			counts[idx]++
		}
	}
	resetCounts := func() {
		for i := range counts {
			counts[i] = 0
		}
	}
	u := func(_ context.Context, _ uint32, _ peer.AddrInfo, _ proto.Message) {}

	bootnodePort := testutil.RandomPort()
	bootnode, err := p2p.NewHost(context.Background(), p2p.DHTProtocolID(1), p2p.Port(bootnodePort), p2p.SecureIO(), p2p.Gossip())
	require.NoError(err)
	bootnodeAddr := bootnode.Addresses()

	for i := 0; i < n; i++ {
		port := bootnodePort + i + 2
		cfg := DefaultConfig
		cfg.Host = "127.0.0.1"
		cfg.Port = port
		cfg.BootstrapNodes = []string{bootnodeAddr[0].String()}
		cfg.ReconnectInterval = 150 * time.Second
		var agent Agent
		if i%2 == 0 {
			opt := JoinSubnet(_blockNetwork, _actionNetwork)
			agent = NewAgent(cfg, 1, hash.ZeroHash256, b(uint8(i)), u, opt)
		} else {
			opt := JoinSubnet(_blockNetwork, _consensusNetwork)
			agent = NewAgent(cfg, 1, hash.ZeroHash256, b(uint8(i)), u, opt)
		}
		agent.Start(ctx)
		agents = append(agents, agent)
	}
	time.Sleep(200 * time.Millisecond)

	t.Run("connectedPeers", func(t *testing.T) {
		for i := 0; i < n; i++ {
			if i%2 == 0 {
				peers, err := agents[i].Subnet(_actionNetwork).ConnectedPeers()
				require.NoError(err)
				require.Len(peers, n/2-1)
				peers, err = agents[i].Subnet(_consensusNetwork).ConnectedPeers()
				require.NoError(err)
				require.Len(peers, n/2)
				peers, err = agents[i].Subnet(_blockNetwork).ConnectedPeers()
				require.NoError(err)
				require.Len(peers, n-1)
			} else {
				peers, err := agents[i].Subnet(_actionNetwork).ConnectedPeers()
				require.NoError(err)
				require.Len(peers, n/2)
				peers, err = agents[i].Subnet(_consensusNetwork).ConnectedPeers()
				require.NoError(err)
				require.Len(peers, n/2-1)
				peers, err = agents[i].Subnet(_blockNetwork).ConnectedPeers()
				require.NoError(err)
				require.Len(peers, n-1)
			}
			peers, err := agents[i].Subnet("unknown").ConnectedPeers()
			require.NoError(err)
			require.Len(peers, 0)
			peers, err = agents[i].ConnectedPeers()
			require.NoError(err)
			require.Len(peers, n-1)
		}
	})

	t.Run("broadcastSubscribed", func(t *testing.T) {
		resetCounts()
		for i := 0; i < n; i++ {
			require.NoError(agents[i].Subnet(_blockNetwork).BroadcastOutbound(ctx, &testingpb.TestPayload{
				MsgBody: []byte{uint8(i)},
			}))
		}
		for i := 0; i < n; i++ {
			require.NoError(testutil.WaitUntil(500*time.Millisecond, 3*time.Second, func() (bool, error) {
				mutex.RLock()
				defer mutex.RUnlock()
				// Broadcast message will be skipped by the source node
				return counts[uint8(i)] == n-1, nil
			}))
		}
	})

	t.Run("broadcastUnsubscribed", func(t *testing.T) {
		resetCounts()
		require.NoError(agents[0].Subnet(_consensusNetwork).BroadcastOutbound(ctx, &testingpb.TestPayload{
			MsgBody: []byte{uint8(0)},
		}))
		for i := 1; i < n; i++ {
			if i%2 == 0 {
				require.NoError(testutil.WaitUntil(500*time.Millisecond, 3*time.Second, func() (bool, error) {
					mutex.RLock()
					defer mutex.RUnlock()
					return counts[uint8(i)] == 0, nil
				}))
			} else {
				require.NoError(testutil.WaitUntil(500*time.Millisecond, 3*time.Second, func() (bool, error) {
					mutex.RLock()
					defer mutex.RUnlock()
					return counts[uint8(i)] == 1, nil
				}))
			}
		}
	})

	t.Run("broadcastUnsubscribedWithNoPeers", func(t *testing.T) {
		resetCounts()
		err := agents[0].Subnet("unknown").BroadcastOutbound(ctx, &testingpb.TestPayload{
			MsgBody: []byte{uint8(0)},
		})
		require.True(errors.Is(err, p2p.ErrNoConnectedPeers))
	})
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
		src = peer.ID.String()
	}
	chainID := uint32(1)
	bootnodePort := testutil.RandomPort()
	bootnode, err := p2p.NewHost(context.Background(), p2p.DHTProtocolID(chainID), p2p.Port(bootnodePort), p2p.SecureIO(), p2p.MasterKey("bootnode"))
	r.NoError(err)
	addrs := bootnode.Addresses()
	for i := 0; i < n; i++ {
		port := bootnodePort + i + 1
		agent := NewAgent(Config{
			Host:              "127.0.0.1",
			Port:              port,
			BootstrapNodes:    []string{addrs[0].String()},
			ReconnectInterval: 150 * time.Second,
			MasterKey:         strconv.Itoa(i),
			MaxMessageSize:    p2p.DefaultConfig.MaxMessageSize,
		}, chainID, hash.ZeroHash256, b, u, JoinSubnet(_blockNetwork))
		r.NoError(agent.Start(ctx))
		agents = append(agents, agent)
	}

	for i := 0; i < n; i++ {
		var (
			neighbors []peer.AddrInfo
			err       error
		)
		err = testutil.WaitUntil(100*time.Millisecond, 2*time.Second, func() (bool, error) {
			neighbors, err = agents[i].Subnet(_blockNetwork).ConnectedPeers()
			return len(neighbors) >= n/3, err
		})
		r.NoError(err)
		for _, neighbor := range neighbors {
			r.NoError(agents[i].Subnet(_blockNetwork).UnicastOutbound(ctx, neighbor, &testingpb.TestPayload{
				MsgBody: []byte{uint8(i)},
			}))
		}
		r.NoError(testutil.WaitUntil(100*time.Millisecond, 20*time.Second, func() (bool, error) {
			mutex.RLock()
			defer mutex.RUnlock()
			info, err := agents[i].Info()
			r.NoError(err)
			return counts[uint8(i)] == len(neighbors) && src == info.ID.String(), nil
		}))
	}
}
