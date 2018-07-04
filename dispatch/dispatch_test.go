// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package dispatch

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/dispatch/dispatcher"
	"github.com/iotexproject/iotex-core/network/node"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/test/mock/mock_blocksync"
	"github.com/iotexproject/iotex-core/test/mock/mock_consensus"
)

func TestNewDispatcher(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	d, bs := createDispatcher(ctrl)
	assert.NotNil(t, d)

	bs.EXPECT().Start().Times(1)
	bs.EXPECT().Stop().Times(1)
	d.Start()
	defer d.Stop()
}

func TestDispatchBlockMsg(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	d, bs := createDispatcher(ctrl)
	assert.NotNil(t, d)

	bs.EXPECT().Start().Times(1)
	bs.EXPECT().Stop().Times(1)
	d.Start()
	defer d.Stop()

	done := make(chan bool, 1000)
	bs.EXPECT().ProcessBlock(gomock.Any()).Times(1000).Return(nil)
	for i := 0; i < 1000; i++ {
		d.HandleBroadcast(&iproto.BlockPb{}, done)
	}
	for i := 0; i < 1000; i++ {
		<-done
	}
}

func TestDispatchBlockSyncReq(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	d, bs := createDispatcher(ctrl)
	assert.NotNil(t, d)

	bs.EXPECT().Start().Times(1)
	bs.EXPECT().Stop().Times(1)
	d.Start()
	defer d.Stop()

	done := make(chan bool, 1000)
	bs.EXPECT().ProcessSyncRequest(gomock.Any(), gomock.Any()).Times(1000).Return(nil)
	for i := 0; i < 1000; i++ {
		d.HandleTell(node.NewTCPNode("192.168.0.0:10000"), &iproto.BlockSync{}, done)
	}
	for i := 0; i < 1000; i++ {
		<-done
	}
}

func TestDispatchBlockSyncData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	d, bs := createDispatcher(ctrl)
	assert.NotNil(t, d)

	bs.EXPECT().Start().Times(1)
	bs.EXPECT().Stop().Times(1)
	d.Start()
	defer d.Stop()

	done := make(chan bool, 1000)
	bs.EXPECT().ProcessBlockSync(gomock.Any()).Times(1000).Return(nil)
	for i := 0; i < 1000; i++ {
		d.HandleTell(node.NewTCPNode("192.168.0.0:10000"), &iproto.BlockContainer{Block: &iproto.BlockPb{}}, done)
	}
	for i := 0; i < 1000; i++ {
		<-done
	}
}

func createDispatcher(
	ctrl *gomock.Controller,
) (dispatcher.Dispatcher, *mock_blocksync.MockBlockSync) {
	cfg := &config.Config{
		Consensus:  config.Consensus{Scheme: config.NOOPScheme},
		Dispatcher: config.Dispatcher{EventChanSize: 1024},
	}
	bs := mock_blocksync.NewMockBlockSync(ctrl)
	cs := mock_consensus.NewMockConsensus(ctrl)
	cs.EXPECT().Start().Times(1)
	cs.EXPECT().Stop().Times(1)
	dp, _ := NewDispatcher(cfg, nil, bs, cs)

	return dp, bs
}
