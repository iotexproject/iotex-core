// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package dispatcher

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/protogen/iotexrpc"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
	"github.com/iotexproject/iotex-core/protogen/testingpb"
)

func createDispatcher(t *testing.T, chainID uint32) Dispatcher {
	cfg := config.Config{
		Consensus:  config.Consensus{Scheme: config.NOOPScheme},
		Dispatcher: config.Dispatcher{EventChanSize: 1024},
	}
	dp, err := NewDispatcher(cfg)
	assert.NoError(t, err)
	dp.AddSubscriber(chainID, &DummySubscriber{})
	return dp
}

func startDispatcher(t *testing.T) (ctx context.Context, d Dispatcher) {
	ctx = context.Background()
	d = createDispatcher(t, config.Default.Chain.ID)
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
			d.HandleBroadcast(ctx, config.Default.Chain.ID, msg)
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
			d.HandleTell(ctx, config.Default.Chain.ID, peerstore.PeerInfo{}, msg)
		}
	}
}

type DummySubscriber struct{}

func (s *DummySubscriber) HandleBlock(context.Context, *iotextypes.Block) error { return nil }

func (s *DummySubscriber) HandleBlockSync(context.Context, *iotextypes.Block) error { return nil }

func (s *DummySubscriber) HandleSyncRequest(context.Context, peerstore.PeerInfo, *iotexrpc.BlockSync) error {
	return nil
}

func (s *DummySubscriber) HandleAction(context.Context, *iotextypes.Action) error { return nil }

func (s *DummySubscriber) HandleConsensusMsg(*iotextypes.ConsensusMessage) error { return nil }
