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
	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/network/node"
	pb "github.com/iotexproject/iotex-core/proto"
)

func TestNewDispatcher(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	d := createDispatcher(config.Default.Chain.ID, ctrl)
	assert.NotNil(t, d)

	err := d.Start(ctx)
	assert.NoError(t, err)
	defer func() {
		err := d.Stop(ctx)
		assert.NoError(t, err)
	}()
}

func TestDispatchBlockMsg(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	d := createDispatcher(config.Default.Chain.ID, ctrl)
	assert.NotNil(t, d)

	err := d.Start(ctx)
	assert.NoError(t, err)
	defer func() {
		err := d.Stop(ctx)
		assert.NoError(t, err)
	}()

	done := make(chan bool, 1000)
	for i := 0; i < 1000; i++ {
		d.HandleBroadcast(config.Default.Chain.ID, &pb.BlockPb{}, done)
	}
	for i := 0; i < 1000; i++ {
		<-done
	}
}

func TestDispatchBlockSyncReq(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	d := createDispatcher(config.Default.Chain.ID, ctrl)
	assert.NotNil(t, d)

	err := d.Start(ctx)
	assert.NoError(t, err)
	defer func() {
		err := d.Stop(ctx)
		assert.NoError(t, err)
	}()

	done := make(chan bool, 1000)
	for i := 0; i < 1000; i++ {
		d.HandleTell(config.Default.Chain.ID, node.NewTCPNode("192.168.0.0:10000"), &pb.BlockSync{}, done)
	}
	for i := 0; i < 1000; i++ {
		<-done
	}
}

func TestDispatchBlockSyncData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	d := createDispatcher(config.Default.Chain.ID, ctrl)
	assert.NotNil(t, d)

	err := d.Start(ctx)
	assert.NoError(t, err)
	defer func() {
		err := d.Stop(ctx)
		assert.NoError(t, err)
	}()

	done := make(chan bool, 1000)
	for i := 0; i < 1000; i++ {
		d.HandleTell(
			config.Default.Chain.ID,
			node.NewTCPNode("192.168.0.0:10000"),
			&pb.BlockContainer{Block: &pb.BlockPb{}},
			done,
		)
	}
	for i := 0; i < 1000; i++ {
		<-done
	}
}

type DummySubscriber struct {
}

func (s *DummySubscriber) HandleBlock(*pb.BlockPb) error {
	return nil
}

func (s *DummySubscriber) HandleBlockSync(*pb.BlockPb) error {
	return nil
}

func (s *DummySubscriber) HandleSyncRequest(string, *pb.BlockSync) error {
	return nil
}

func (s *DummySubscriber) HandleAction(*pb.ActionPb) error {
	return nil
}

func (s *DummySubscriber) HandleViewChange(proto.Message) error {
	return nil
}

func (s *DummySubscriber) HandleBlockPropose(proto.Message) error {
	return nil
}

func createDispatcher(
	chainID uint32,
	ctrl *gomock.Controller,
) Dispatcher {
	cfg := &config.Config{
		Consensus:  config.Consensus{Scheme: config.NOOPScheme},
		Dispatcher: config.Dispatcher{EventChanSize: 1024},
	}
	dp, _ := NewDispatcher(cfg)
	dp.AddSubscriber(chainID, &DummySubscriber{})
	return dp
}
