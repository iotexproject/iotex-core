// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package dispatcher

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/iotexproject/iotex-proto/golang/testingpb"
)

// TODO: define defaultChainID in chain.DefaultConfig
var (
	defaultChainID uint32 = 1
)

func createDispatcher(t *testing.T, chainID uint32) Dispatcher {
	dp, err := NewDispatcher(DefaultConfig)
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
