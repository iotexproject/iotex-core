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
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-p2p"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/iotexproject/iotex-proto/golang/testingpb"

	"github.com/iotexproject/iotex-core/v2/testutil"
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
			MaxMessageSize:    p2p.DefaultConfig.MaxMessageSize,
		}, 1, hash.ZeroHash256, func(proto.Message) (bool, error) {
			return false, nil
		}, b, u)
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
		src = peer.ID.String()
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
		}, 2, hash.ZeroHash256, func(proto.Message) (bool, error) {
			return false, nil
		}, b, u)
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
			return counts[uint8(i)] == len(neighbors) && src == info.ID.String(), nil
		}))
	}
}

func TestConsensusTopicMsgTypes(t *testing.T) {
	r := require.New(t)
	for _, unified := range []bool{false, true} {
		p := &agent{}
		p.unifiedTopic.Store(unified)
		consensusTopic := _broadcastTopic + _broadcastSubTopicConsensus + p.topicSuffix
		for v := range iotexrpc.MessageType_name {
			mt := iotexrpc.MessageType(v)
			// every type routed to the consensus topic must pass its validator
			if p.messageTopic(mt) == consensusTopic {
				r.True(isConsensusTopicMsgType(mt), mt.String())
			}
		}
	}
	r.True(isConsensusTopicMsgType(iotexrpc.MessageType_CONSENSUS))
	for _, mt := range []iotexrpc.MessageType{
		iotexrpc.MessageType_ACTION,
		iotexrpc.MessageType_ACTIONS,
		iotexrpc.MessageType_BLOCK,
		iotexrpc.MessageType_BUNDLE,
	} {
		r.False(isConsensusTopicMsgType(mt), mt.String())
	}
}

func TestConnectBootNodes(t *testing.T) {
	addrs := make([]multiaddr.Multiaddr, 6)
	for i := range addrs {
		addrs[i] = multiaddr.StringCast("/ip4/127.0.0.1/tcp/" + strconv.Itoa(4000+i))
	}
	// dialer that succeeds for the first n addresses and fails for the rest
	dialFirst := func(n int) func(context.Context, multiaddr.Multiaddr) error {
		return func(_ context.Context, a multiaddr.Multiaddr) error {
			for _, ok := range addrs[:n] {
				if ok.Equal(a) {
					return nil
				}
			}
			return errors.New("refused")
		}
	}
	// run connectBootNodes and fail the test if it does not return in time
	run := func(t *testing.T, ctx context.Context, addrs []multiaddr.Multiaddr, dial func(context.Context, multiaddr.Multiaddr) error) error {
		done := make(chan error, 1)
		go func() { done <- connectBootNodes(ctx, addrs, dial) }()
		select {
		case err := <-done:
			return err
		case <-time.After(3 * time.Second):
			t.Fatal("connectBootNodes did not return")
			return nil
		}
	}

	t.Run("no bootnode", func(t *testing.T) {
		require.NoError(t, run(t, context.Background(), nil, dialFirst(0)))
	})
	t.Run("enough connected", func(t *testing.T) {
		require.NoError(t, run(t, context.Background(), addrs, dialFirst(4)))
	})
	t.Run("some connected, some failed", func(t *testing.T) {
		// 3 of 6 are reachable but 4 are wanted: must return instead of waiting forever
		require.NoError(t, run(t, context.Background(), addrs, dialFirst(3)))
	})
	t.Run("all failed", func(t *testing.T) {
		err := run(t, context.Background(), addrs, dialFirst(0))
		require.ErrorContains(t, err, "failed to connect to any bootstrap node")
	})
	t.Run("context canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		block := func(ctx context.Context, _ multiaddr.Multiaddr) error {
			<-ctx.Done()
			return ctx.Err()
		}
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()
		require.ErrorIs(t, run(t, ctx, addrs, block), context.Canceled)
	})
	t.Run("leftover dials are canceled on return", func(t *testing.T) {
		canceled := make(chan struct{})
		dial := func(ctx context.Context, a multiaddr.Multiaddr) error {
			if !a.Equal(addrs[2]) {
				return nil
			}
			<-ctx.Done()
			close(canceled)
			return ctx.Err()
		}
		// 2 of 3 connected is enough, so it returns while the third dial is still running
		require.NoError(t, run(t, context.Background(), addrs[:3], dial))
		select {
		case <-canceled:
		case <-time.After(3 * time.Second):
			t.Fatal("leftover dial was not canceled")
		}
	})
}

func TestExponentialRetryContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := exponentialRetry(ctx, func() error { return errors.New("fail") }, time.Minute, 3)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), 5*time.Second)
}

func TestExponentialRetryNoSleepAfterLastAttempt(t *testing.T) {
	calls := 0
	done := make(chan error, 1)
	go func() {
		done <- exponentialRetry(context.Background(), func() error {
			calls++
			return errors.New("fail")
		}, time.Hour, 1)
	}()
	select {
	case err := <-done:
		require.EqualError(t, err, "fail")
		require.Equal(t, 1, calls)
	case <-time.After(3 * time.Second):
		t.Fatal("exponentialRetry slept after its last attempt")
	}
}

func TestStopCancelsReconnect(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()

	bootnode, err := p2p.NewHost(ctx, p2p.DHTProtocolID(2), p2p.Port(testutil.RandomPort()), p2p.SecureIO(), p2p.MasterKey("bootnode"))
	r.NoError(err)
	b := func(context.Context, uint32, string, proto.Message) {}
	u := func(context.Context, uint32, peer.AddrInfo, proto.Message) {}
	a := NewAgent(Config{
		Host:              "127.0.0.1",
		Port:              testutil.RandomPort(),
		BootstrapNodes:    []string{bootnode.Addresses()[0].String()},
		ReconnectInterval: 100 * time.Millisecond,
		MasterKey:         "agent",
	}, 2, hash.ZeroHash256, func(proto.Message) (bool, error) { return false, nil }, b, u)
	r.NoError(a.Start(ctx))

	// take the only bootnode away, so the next reconnect tick keeps retrying it
	r.NoError(bootnode.Close())
	r.NoError(testutil.WaitUntil(50*time.Millisecond, 10*time.Second, func() (bool, error) {
		peers, err := a.ConnectedPeers()
		return len(peers) == 0, err
	}))
	// let a reconnect start and reach its retry backoff
	time.Sleep(500 * time.Millisecond)

	done := make(chan error, 1)
	go func() { done <- a.Stop(ctx) }()
	select {
	case err := <-done:
		r.NoError(err)
	case <-time.After(5 * time.Second):
		t.Fatal("Stop waited for the reconnect to finish its retries")
	}
}
