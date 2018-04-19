// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package dispatcher

import (
	"net"
	"sync"
	"sync/atomic"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blocksync"
	cm "github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/delegate"
	pb "github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/txpool"
)

// txMsg packages a proto tx message.
type txMsg struct {
	tx   *pb.TxPb
	done chan bool
}

// blockMsg packages a proto block message.
type blockMsg struct {
	block   *pb.BlockPb
	blkType uint32
	done    chan bool
}

// blockSyncMsg packages a proto block sync message.
type blockSyncMsg struct {
	sender string
	sync   *pb.BlockSync
	done   chan bool
}

// dispatcher implements Dispatcher interface.
type dispatcher struct {
	started  int32
	shutdown int32
	newsChan chan interface{}
	wg       sync.WaitGroup
	quit     chan struct{}

	bs blocksync.BlockSync
	cs consensus.Consensus
	tp txpool.TxPool
}

// NewDispatcher creates a new dispatcher
func NewDispatcher(cfg *config.Config, bc blockchain.IBlockchain, tp txpool.TxPool, bs blocksync.BlockSync, dp delegate.Pool) cm.Dispatcher {
	if bc == nil || bs == nil {
		glog.Fatal("Try to attach to a nil blockchain or a nil P2P")
		return nil
	}

	d := &dispatcher{
		newsChan: make(chan interface{}, 1024),
		quit:     make(chan struct{}),
		tp:       tp,
		bs:       bs,
	}

	d.cs = consensus.NewConsensus(cfg, bc, tp, bs, dp)
	return d
}

// Start starts the dispatcher.
func (d *dispatcher) Start() error {
	if atomic.AddInt32(&d.started, 1) != 1 {
		return errors.New("Dispatcher already started")
	}

	glog.Info("Starting dispatcher")
	if err := d.cs.Start(); err != nil {
		return err
	}

	if err := d.bs.Start(); err != nil {
		return err
	}

	d.wg.Add(1)
	go d.newsHandler()
	return nil
}

// Stop gracefully shuts down the dispatcher by stopping all handlers and waiting for them to finish.
func (d *dispatcher) Stop() error {
	if atomic.AddInt32(&d.shutdown, 1) != 1 {
		glog.Warning("Dispatcher already in the process of shutting down")
		return nil
	}

	glog.Infof("Dispatcher is shutting down")
	if err := d.cs.Stop(); err != nil {
		return err
	}

	if err := d.bs.Stop(); err != nil {
		return err
	}

	close(d.quit)
	d.wg.Wait()
	return nil
}

// newsHandler is the main handler for handling all news from peers.
func (d *dispatcher) newsHandler() {
loop:
	for {
		select {
		case m := <-d.newsChan:
			switch msg := m.(type) {
			case *txMsg:
				d.handleTxMsg(msg)

			case *blockMsg:
				d.handleBlockMsg(msg)

			case *blockSyncMsg:
				d.handleBlockSyncMsg(msg)

			default:
				glog.Warning("Invalid message type in block handler: %T", msg)
			}

		case <-d.quit:
			break loop
		}
	}

	d.wg.Done()
	glog.Info("News handler done")
}

// handleTxMsg handles txMsg from all peers.
func (d *dispatcher) handleTxMsg(m *txMsg) {
	tx := &blockchain.Tx{}
	tx.ConvertFromTxPb(m.tx)
	glog.Infof("receive txMsg, hash = %x", tx.Hash())

	// dispatch to TxPool
	if _, err := d.tp.ProcessTx(tx, true, true, 0); err != nil {
		glog.Error(err)
	}

	// signal to let caller know we are done
	if m.done != nil {
		m.done <- true
	}

	return
}

// handleBlockMsg handles blockMsg from peers.
func (d *dispatcher) handleBlockMsg(m *blockMsg) {
	blk := &blockchain.Block{}
	blk.ConvertFromBlockPb(m.block)
	glog.Infof("receive blockMsg, block %d, hash = %x", blk.Height(), blk.HashBlock())

	if m.blkType == pb.MsgBlockProtoMsgType {
		if err := d.bs.ProcessBlock(blk); err != nil {
			glog.Error(err)
		}
	} else if m.blkType == pb.MsgBlockSyncDataType {
		if err := d.bs.ProcessBlockSync(blk); err != nil {
			glog.Error(err)
		}
	}

	// signal to let caller know we are done
	if m.done != nil {
		m.done <- true
	}

	return
}

// handleBlockSyncMsg handles block messages from peers.
func (d *dispatcher) handleBlockSyncMsg(m *blockSyncMsg) {
	glog.Infof("receive blockSyncMsg, addr = %s, start = %d, end = %d", m.sender, m.sync.Start, m.sync.End)

	// dispatch to block sync
	if err := d.bs.ProcessSyncRequest(m.sender, m.sync); err != nil {
		glog.Error(err)
	}

	// signal to let caller know we are done
	if m.done != nil {
		m.done <- true
	}

	return
}

// dispatchTx adds the passed transaction message to the news handling queue.
func (d *dispatcher) dispatchTx(msg proto.Message, done chan bool) {
	if atomic.LoadInt32(&d.shutdown) != 0 {
		if done != nil {
			close(done)
		}
		return
	}

	d.newsChan <- &txMsg{(msg).(*pb.TxPb), done}
}

// dispatchBlockCommit adds the passed block message to the news handling queue.
func (d *dispatcher) dispatchBlockCommit(msg proto.Message, done chan bool) {
	if atomic.LoadInt32(&d.shutdown) != 0 {
		if done != nil {
			close(done)
		}
		return
	}

	d.newsChan <- &blockMsg{(msg).(*pb.BlockPb), pb.MsgBlockProtoMsgType, done}
}

// dispatchBlockSyncReq adds the passed block sync request to the news handling queue.
func (d *dispatcher) dispatchBlockSyncReq(sender string, msg proto.Message, done chan bool) {
	if atomic.LoadInt32(&d.shutdown) != 0 {
		if done != nil {
			close(done)
		}
		return
	}

	d.newsChan <- &blockSyncMsg{sender, (msg).(*pb.BlockSync), done}
}

// dispatchBlockSyncData handles block sync data
func (d *dispatcher) dispatchBlockSyncData(msg proto.Message, done chan bool) {
	if atomic.LoadInt32(&d.shutdown) != 0 {
		if done != nil {
			close(done)
		}
		return
	}

	data := (msg).(*pb.BlockContainer)
	d.newsChan <- &blockMsg{data.Block, pb.MsgBlockSyncDataType, done}
}

// HandleBroadcast handles incoming broadcast message

func (d *dispatcher) HandleBroadcast(message proto.Message, done chan bool) {
	msgType, err := pb.GetTypeFromProtoMsg(message)
	if err != nil {
		glog.Warning("unexpected message handled by HandleBroadcast: ", err.Error())
	}

	switch msgType {
	case pb.ViewChangeMsgType:
		d.cs.HandleViewChange(message, done)
		break
	case pb.MsgTxProtoMsgType:
		d.dispatchTx(message, done)
		break
	case pb.MsgBlockProtoMsgType:
		d.dispatchBlockCommit(message, done)
		break
	default:
		glog.Warning("unexpected msgType %v handled by HandleBroadcast", msgType)
	}
}

// HandleTell handles incoming unicast message
func (d *dispatcher) HandleTell(sender net.Addr, message proto.Message, done chan bool) {
	msgType, err := pb.GetTypeFromProtoMsg(message)
	if err != nil {
		glog.Warning("unexpected message handled by HandleTell: ", err.Error())
	}

	glog.Info("dispatcher.HandleTell from", sender, message)
	switch msgType {
	case pb.MsgBlockSyncReqType:
		d.dispatchBlockSyncReq(sender.String(), message, done)
	case pb.MsgBlockSyncDataType:
		d.dispatchBlockSyncData(message, done)
	case pb.MsgBlockProtoMsgType:
		d.cs.HandleBlockPropose(message, done)
	default:
		glog.Warning("unexpected msgType %v handled by HandleTell", msgType)
	}
}
