// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package dispatcher

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/iotexproject/iotex-proto/golang/testingpb"

	"github.com/iotexproject/iotex-core/v2/testutil"
)

// TODO: define defaultChainID in chain.DefaultConfig
var (
	defaultChainID        uint32 = 1
	dummyVerificationFunc        = func(proto.Message) (string, error) { return "", nil }
)

func createDispatcher(t *testing.T, chainID uint32) Dispatcher {
	dp, err := NewDispatcher(DefaultConfig, dummyVerificationFunc)
	assert.NoError(t, err)
	dp.AddSubscriber(chainID, &dummySubscriber{})
	return dp
}

func startDispatcher(t *testing.T) (ctx context.Context, d Dispatcher) {
	ctx = context.Background()
	d = createDispatcher(t, defaultChainID)
	assert.NotNil(t, d)
	err := d.Start(ctx)
	assert.NoError(t, err)
	return
}

func stopDispatcher(ctx context.Context, d Dispatcher, t *testing.T) {
	err := d.Stop(ctx)
	assert.NoError(t, err)
}

func setTestCase() []proto.Message {
	return []proto.Message{
		&iotextypes.Action{},
		&iotextypes.Actions{
			Actions: []*iotextypes.Action{
				&iotextypes.Action{},
			},
		},
		&iotextypes.ConsensusMessage{},
		&iotextypes.Block{},
		&iotexrpc.BlockSync{},
		&testingpb.TestPayload{},
		&iotextypes.NodeInfoRequest{},
		&iotextypes.NodeInfo{},
	}
}

func TestHandleBroadcast(t *testing.T) {
	msgs := setTestCase()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, d := startDispatcher(t)
	defer stopDispatcher(ctx, d, t)

	for i := 0; i < 100; i++ {
		for _, msg := range msgs {
			d.HandleBroadcast(ctx, defaultChainID, "peer1", msg)
		}
	}
}

func TestHandleTell(t *testing.T) {
	msgs := setTestCase()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, d := startDispatcher(t)
	defer stopDispatcher(ctx, d, t)

	for i := 0; i < 100; i++ {
		for _, msg := range msgs {
			d.HandleTell(ctx, defaultChainID, peer.AddrInfo{}, msg)
		}
	}
}

func TestDispatcherV2(t *testing.T) {
	r := require.New(t)
	t.Run("limitBlockSync", func(t *testing.T) {
		cfg := DefaultConfig
		cfg.ProcessSyncRequestInterval = time.Second
		dsp, err := NewDispatcher(cfg, dummyVerificationFunc)
		r.NoError(err)
		r.NoError(dsp.Start(context.Background()))
		defer func() {
			r.NoError(dsp.Stop(context.Background()))
		}()
		peer1 := peer.AddrInfo{ID: "peer1"}
		peer2 := peer.AddrInfo{ID: "peer2"}
		sub := &counterSubscriber{}
		dsp.AddSubscriber(defaultChainID, sub)
		dsp.HandleTell(context.Background(), defaultChainID, peer1, &iotexrpc.BlockSync{})
		dsp.HandleTell(context.Background(), defaultChainID, peer2, &iotexrpc.BlockSync{})
		dsp.HandleTell(context.Background(), defaultChainID, peer2, &iotexrpc.BlockSync{})
		r.NoError(testutil.WaitUntil(100*time.Millisecond, time.Second, func() (bool, error) {
			return dispatcherIsClean(dsp.(*IotxDispatcher)), nil
		}))
		r.Equal(int32(2), sub.blockSync.Load())
	})
	t.Run("broadcast", func(t *testing.T) {
		dsp, err := NewDispatcher(DefaultConfig, dummyVerificationFunc)
		r.NoError(err)
		r.NoError(dsp.Start(context.Background()))
		defer func() {
			r.NoError(dsp.Stop(context.Background()))
		}()
		// add subscriber
		sub := &counterSubscriber{}
		dsp.AddSubscriber(defaultChainID, sub)
		// Test handle broadcast
		cases := setTestCase()
		for _, msg := range cases {
			dsp.HandleBroadcast(context.Background(), defaultChainID, "peer1", msg)
		}
		r.NoError(testutil.WaitUntil(100*time.Millisecond, time.Second, func() (bool, error) {
			return dispatcherIsClean(dsp.(*IotxDispatcher)), nil
		}))
		r.Equal(int32(2), sub.action.Load())
		r.Equal(int32(1), sub.block.Load())
		r.Equal(int32(1), sub.consensus.Load())
		r.Equal(int32(1), sub.nodeInfo.Load())
	})
	t.Run("unicast", func(t *testing.T) {
		dsp, err := NewDispatcher(DefaultConfig, dummyVerificationFunc)
		r.NoError(err)
		r.NoError(dsp.Start(context.Background()))
		defer func() {
			r.NoError(dsp.Stop(context.Background()))
		}()
		// add subscriber
		sub := &counterSubscriber{}
		dsp.AddSubscriber(defaultChainID, sub)
		// Test handle broadcast
		cases := setTestCase()
		for _, msg := range cases {
			dsp.HandleTell(context.Background(), defaultChainID, peer.AddrInfo{}, msg)
		}
		r.NoError(testutil.WaitUntil(100*time.Millisecond, time.Second, func() (bool, error) {
			return dispatcherIsClean(dsp.(*IotxDispatcher)), nil
		}))
		r.Equal(int32(1), sub.blockSync.Load())
		r.Equal(int32(1), sub.nodeInfoReq.Load())
		r.Equal(int32(1), sub.nodeInfo.Load())
		r.Equal(int32(1), sub.block.Load())
	})
}

func dispatcherIsClean(dsp *IotxDispatcher) bool {
	for k := range dsp.queueMgr.queues {
		if len(dsp.queueMgr.queues[k]) != 0 {
			return false
		}
	}
	return true
}

type dummySubscriber struct{}

func (ds *dummySubscriber) ReportFullness(context.Context, iotexrpc.MessageType, float32) {}

func (ds *dummySubscriber) HandleBlock(context.Context, string, *iotextypes.Block) error { return nil }

func (ds *dummySubscriber) HandleSyncRequest(context.Context, peer.AddrInfo, *iotexrpc.BlockSync) error {
	return nil
}

func (ds *dummySubscriber) HandleAction(context.Context, *iotextypes.Action) error { return nil }

func (ds *dummySubscriber) HandleConsensusMsg(*iotextypes.ConsensusMessage) error { return nil }

func (ds *dummySubscriber) HandleNodeInfoRequest(context.Context, peer.AddrInfo, *iotextypes.NodeInfoRequest) error {
	return nil
}

func (ds *dummySubscriber) HandleNodeInfo(context.Context, string, *iotextypes.NodeInfo) error {
	return nil
}

func (ds *dummySubscriber) HandleActionRequest(context.Context, peer.AddrInfo, hash.Hash256) error {
	return nil
}

func (ds *dummySubscriber) HandleActionHash(context.Context, hash.Hash256, string) error {
	return nil
}

type counterSubscriber struct {
	block       atomic.Int32
	blockSync   atomic.Int32
	action      atomic.Int32
	consensus   atomic.Int32
	nodeInfo    atomic.Int32
	nodeInfoReq atomic.Int32
	actionReq   atomic.Int32
	actionHash  atomic.Int32
}

func (cs *counterSubscriber) ReportFullness(context.Context, iotexrpc.MessageType, float32) {}

func (cs *counterSubscriber) HandleBlock(context.Context, string, *iotextypes.Block) error {
	cs.block.Inc()
	return nil
}

func (cs *counterSubscriber) HandleSyncRequest(context.Context, peer.AddrInfo, *iotexrpc.BlockSync) error {
	cs.blockSync.Inc()
	return nil
}

func (cs *counterSubscriber) HandleAction(context.Context, *iotextypes.Action) error {
	cs.action.Inc()
	return nil
}

func (cs *counterSubscriber) HandleConsensusMsg(*iotextypes.ConsensusMessage) error {
	cs.consensus.Inc()
	return nil
}

func (cs *counterSubscriber) HandleNodeInfoRequest(context.Context, peer.AddrInfo, *iotextypes.NodeInfoRequest) error {
	cs.nodeInfoReq.Inc()
	return nil
}

func (cs *counterSubscriber) HandleNodeInfo(context.Context, string, *iotextypes.NodeInfo) error {
	cs.nodeInfo.Inc()
	return nil
}

func (cs *counterSubscriber) HandleActionRequest(context.Context, peer.AddrInfo, hash.Hash256) error {
	cs.actionReq.Inc()
	return nil
}

func (cs *counterSubscriber) HandleActionHash(context.Context, hash.Hash256, string) error {
	cs.actionHash.Inc()
	return nil
}
