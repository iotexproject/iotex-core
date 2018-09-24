// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package dispatch

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/dispatch/dispatcher"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/network/node"
	"github.com/iotexproject/iotex-core/proto"
	pb "github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/test/mock/mock_blocksync"
)

func TestNewDispatcher(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	chainID := iotxaddress.MainChainID()
	d, _ := createDispatcher(chainID, ctrl)
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
	chainID := iotxaddress.MainChainID()
	d, bs := createDispatcher(chainID, ctrl)
	assert.NotNil(t, d)

	err := d.Start(ctx)
	assert.NoError(t, err)
	defer func() {
		err := d.Stop(ctx)
		assert.NoError(t, err)
	}()

	done := make(chan bool, 1000)
	bs.EXPECT().ProcessBlock(gomock.Any()).Times(1000).Return(nil)
	for i := 0; i < 1000; i++ {
		d.HandleBroadcast(chainID, &iproto.BlockPb{}, done)
	}
	for i := 0; i < 1000; i++ {
		<-done
	}
}

func TestDispatchBlockSyncReq(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	chainID := iotxaddress.MainChainID()
	d, bs := createDispatcher(chainID, ctrl)
	assert.NotNil(t, d)

	err := d.Start(ctx)
	assert.NoError(t, err)
	defer func() {
		err := d.Stop(ctx)
		assert.NoError(t, err)
	}()

	done := make(chan bool, 1000)
	bs.EXPECT().ProcessSyncRequest(gomock.Any(), gomock.Any()).Times(1000).Return(nil)
	for i := 0; i < 1000; i++ {
		d.HandleTell(chainID, node.NewTCPNode("192.168.0.0:10000"), &iproto.BlockSync{}, done)
	}
	for i := 0; i < 1000; i++ {
		<-done
	}
}

func TestDispatchBlockSyncData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	chainID := iotxaddress.MainChainID()
	d, bs := createDispatcher(chainID, ctrl)
	assert.NotNil(t, d)

	err := d.Start(ctx)
	assert.NoError(t, err)
	defer func() {
		err := d.Stop(ctx)
		assert.NoError(t, err)
	}()

	done := make(chan bool, 1000)
	bs.EXPECT().ProcessBlockSync(gomock.Any()).Times(1000).Return(nil)
	for i := 0; i < 1000; i++ {
		d.HandleTell(chainID, node.NewTCPNode("192.168.0.0:10000"), &iproto.BlockContainer{Block: &iproto.BlockPb{}}, done)
	}
	for i := 0; i < 1000; i++ {
		<-done
	}
}

type MockSubscriber struct {
	bs blocksync.BlockSync
}

func (bcs *MockSubscriber) HandleAction(act *pb.ActionPb) error {
	return nil
}

func (bcs *MockSubscriber) HandleBlock(blk *blockchain.Block) error {
	return bcs.bs.ProcessBlock(blk)
}

func (bcs *MockSubscriber) HandleBlockSync(blk *blockchain.Block) error {
	return bcs.bs.ProcessBlockSync(blk)
}

func (bcs *MockSubscriber) HandleSyncRequest(sender string, sync *pb.BlockSync) error {
	return bcs.bs.ProcessSyncRequest(sender, sync)
}

func (bcs *MockSubscriber) HandleViewChange(msg proto.Message) error {
	return nil
}

func (bcs *MockSubscriber) HandleBlockPropose(msg proto.Message) error {
	return nil
}

func createDispatcher(
	chainID uint32,
	ctrl *gomock.Controller,
) (dispatcher.Dispatcher, *mock_blocksync.MockBlockSync) {
	cfg := &config.Config{
		Consensus:  config.Consensus{Scheme: config.NOOPScheme},
		Dispatcher: config.Dispatcher{EventChanSize: 1024},
	}
	bs := mock_blocksync.NewMockBlockSync(ctrl)
	dp, _ := NewDispatcher(cfg)
	dp.AddSubscriber(chainID, &MockSubscriber{bs})

	return dp, bs
}
