// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package dispatcher

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	cm "github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_blocksync"
	"github.com/iotexproject/iotex-core/test/mock/mock_delegate"
	"github.com/iotexproject/iotex-core/test/mock/mock_txpool"
)

func TestNewDispatcher(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{Consensus: config.Consensus{Scheme: config.NOOPScheme}}
	bc := mock_blockchain.NewMockBlockchain(ctrl)
	tp := mock_txpool.NewMockTxPool(ctrl)
	bs := mock_blocksync.NewMockBlockSync(ctrl)
	dp := mock_delegate.NewMockPool(ctrl)

	d := NewDispatcher(cfg, bc, tp, nil, bs, dp)
	assert.NotNil(t, d)

	bs.EXPECT().Start().Times(1)
	bs.EXPECT().Stop().Times(1)
	d.Start()
	defer d.Stop()
}

func TestDispatchTxMsg(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{Consensus: config.Consensus{Scheme: config.NOOPScheme}}
	bc := mock_blockchain.NewMockBlockchain(ctrl)
	tp := mock_txpool.NewMockTxPool(ctrl)
	bs := mock_blocksync.NewMockBlockSync(ctrl)
	dp := mock_delegate.NewMockPool(ctrl)

	d := NewDispatcher(cfg, bc, tp, nil, bs, dp)
	assert.NotNil(t, d)

	bs.EXPECT().Start().Times(1)
	bs.EXPECT().Stop().Times(1)
	d.Start()
	defer d.Stop()

	done := make(chan bool, 1000)
	tp.EXPECT().ProcessTx(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1000).Return(nil, nil)
	for i := 0; i < 1000; i++ {
		d.HandleBroadcast(&iproto.TxPb{}, done)
	}
	for i := 0; i < 1000; i++ {
		<-done
	}
}

func TestDispatchBlockMsg(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{Consensus: config.Consensus{Scheme: config.NOOPScheme}}
	bc := mock_blockchain.NewMockBlockchain(ctrl)
	tp := mock_txpool.NewMockTxPool(ctrl)
	bs := mock_blocksync.NewMockBlockSync(ctrl)
	dp := mock_delegate.NewMockPool(ctrl)

	d := NewDispatcher(cfg, bc, tp, nil, bs, dp)
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

	cfg := &config.Config{Consensus: config.Consensus{Scheme: config.NOOPScheme}}
	bc := mock_blockchain.NewMockBlockchain(ctrl)
	tp := mock_txpool.NewMockTxPool(ctrl)
	bs := mock_blocksync.NewMockBlockSync(ctrl)
	dp := mock_delegate.NewMockPool(ctrl)

	d := NewDispatcher(cfg, bc, tp, nil, bs, dp)
	assert.NotNil(t, d)

	bs.EXPECT().Start().Times(1)
	bs.EXPECT().Stop().Times(1)
	d.Start()
	defer d.Stop()

	done := make(chan bool, 1000)
	bs.EXPECT().ProcessSyncRequest(gomock.Any(), gomock.Any()).Times(1000).Return(nil)
	for i := 0; i < 1000; i++ {
		d.HandleTell(cm.NewTCPNode("192.168.0.0:10000"), &iproto.BlockSync{}, done)
	}
	for i := 0; i < 1000; i++ {
		<-done
	}
}

func TestDispatchBlockSyncData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{Consensus: config.Consensus{Scheme: config.NOOPScheme}}
	bc := mock_blockchain.NewMockBlockchain(ctrl)
	tp := mock_txpool.NewMockTxPool(ctrl)
	bs := mock_blocksync.NewMockBlockSync(ctrl)
	dp := mock_delegate.NewMockPool(ctrl)

	d := NewDispatcher(cfg, bc, tp, nil, bs, dp)
	assert.NotNil(t, d)

	bs.EXPECT().Start().Times(1)
	bs.EXPECT().Stop().Times(1)
	d.Start()
	defer d.Stop()

	done := make(chan bool, 1000)
	bs.EXPECT().ProcessBlockSync(gomock.Any()).Times(1000).Return(nil)
	for i := 0; i < 1000; i++ {
		d.HandleTell(cm.NewTCPNode("192.168.0.0:10000"), &iproto.BlockContainer{Block: &iproto.BlockPb{}}, done)
	}
	for i := 0; i < 1000; i++ {
		<-done
	}
}
