// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package dispatch

import (
	"net"
	"sync"
	"sync/atomic"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/blockchain/trx"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/dispatch/dispatcher"
	"github.com/iotexproject/iotex-core/logger"
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

// actionMsg packages a proto action message.
type actionMsg struct {
	action *pb.ActionPb
	done   chan bool
}

// iotxDispatcher is the request and event dispatcher for iotx node.
type iotxDispatcher struct {
	started  int32
	shutdown int32
	newsChan chan interface{}
	wg       sync.WaitGroup
	quit     chan struct{}

	bs blocksync.BlockSync
	cs consensus.Consensus
	tp txpool.TxPool
	ap txpool.ActPool
}

// NewDispatcher creates a new iotxDispatcher
func NewDispatcher(cfg *config.Config, bc blockchain.Blockchain, tp txpool.TxPool, ap txpool.ActPool,
	bs blocksync.BlockSync, dp delegate.Pool) dispatcher.Dispatcher {
	if bc == nil || bs == nil {
		logger.Error().Msg("Try to attach to a nil blockchain or a nil P2P")
		return nil
	}
	d := &iotxDispatcher{
		newsChan: make(chan interface{}, 1024),
		quit:     make(chan struct{}),
		tp:       tp,
		ap:       ap,
		bs:       bs,
	}
	d.cs = consensus.NewConsensus(cfg, bc, tp, ap, bs, dp)
	return d
}

// Start starts the dispatcher.
func (d *iotxDispatcher) Start() error {
	if atomic.AddInt32(&d.started, 1) != 1 {
		return errors.New("Dispatcher already started")
	}

	logger.Info().Msg("Starting dispatcher")
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
func (d *iotxDispatcher) Stop() error {
	if atomic.AddInt32(&d.shutdown, 1) != 1 {
		logger.Warn().Msg("Dispatcher already in the process of shutting down")
		return nil
	}

	logger.Info().Msg("Dispatcher is shutting down")
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
func (d *iotxDispatcher) newsHandler() {
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

			case *actionMsg:
				d.handleActionMsg(msg)

			default:
				logger.Warn().
					Str("msg", msg.(string)).
					Msg("Invalid message type in block handler")
			}

		case <-d.quit:
			break loop
		}
	}

	d.wg.Done()
	logger.Info().Msg("News handler done")
}

// handleTxMsg handles txMsg from all peers.
func (d *iotxDispatcher) handleTxMsg(m *txMsg) {
	tx := &trx.Tx{}
	tx.ConvertFromTxPb(m.tx)
	x := tx.Hash()
	logger.Info().Hex("hash", x[:]).Msg("receive txMsg")
	// dispatch to TxPool
	if _, err := d.tp.ProcessTx(tx, true, true, 0); err != nil {
		logger.Error().Err(err)
	}
	// signal to let caller know we are done
	if m.done != nil {
		m.done <- true
	}
}

// handleBlockMsg handles blockMsg from peers.
func (d *iotxDispatcher) handleBlockMsg(m *blockMsg) {
	blk := &blockchain.Block{}
	blk.ConvertFromBlockPb(m.block)
	hash := blk.HashBlock()
	logger.Info().
		Uint64("block", blk.Height()).Hex("hash", hash[:]).Msg("receive blockMsg")

	if m.blkType == pb.MsgBlockProtoMsgType {
		if err := d.bs.ProcessBlock(blk); err != nil {
			logger.Error().Err(err)
		}
	} else if m.blkType == pb.MsgBlockSyncDataType {
		if err := d.bs.ProcessBlockSync(blk); err != nil {
			logger.Error().Err(err)
		}
	}
	// signal to let caller know we are done
	if m.done != nil {
		m.done <- true
	}
}

// handleBlockSyncMsg handles block messages from peers.
func (d *iotxDispatcher) handleBlockSyncMsg(m *blockSyncMsg) {
	logger.Info().
		Str("addr", m.sender).Uint64("start", m.sync.Start).Uint64("end", m.sync.End).
		Msg("receive blockSyncMsg")
	// dispatch to block sync
	if err := d.bs.ProcessSyncRequest(m.sender, m.sync); err != nil {
		logger.Error().Err(err)
	}
	// signal to let caller know we are done
	if m.done != nil {
		m.done <- true
	}
}

// handleActionMsg handles actionMsg from all peers.
func (d *iotxDispatcher) handleActionMsg(m *actionMsg) {
	vote := &pb.VotePb{}
	logger.Info().Str("sig", string(vote.Signature)).Msg("receive actionMsg")

	// dispatch to ActPool
	if pbTsf := m.action.GetTransfer(); pbTsf != nil {
		tsf := &action.Transfer{}
		tsf.ConvertFromTransferPb(pbTsf)
		if err := d.ap.AddTsf(tsf); err != nil {
			logger.Error().Err(err)
		}
		// TODO: defer m.done and return error to caller
		return
	}
	if pbVote := m.action.GetVote(); pbVote != nil {
		vote := &action.Vote{}
		vote.ConvertFromVotePb(pbVote)
		if err := d.ap.AddVote(vote); err != nil {
			logger.Error().Err(err)
		}
		// TODO: defer m.done and return error to caller
		return
	}
	// signal to let caller know we are done
	if m.done != nil {
		m.done <- true
	}
}

// dispatchTx adds the passed transaction message to the news handling queue.
func (d *iotxDispatcher) dispatchTx(msg proto.Message, done chan bool) {
	if atomic.LoadInt32(&d.shutdown) != 0 {
		if done != nil {
			close(done)
		}
		return
	}
	d.newsChan <- &txMsg{(msg).(*pb.TxPb), done}
}

// dispatchBlockCommit adds the passed block message to the news handling queue.
func (d *iotxDispatcher) dispatchBlockCommit(msg proto.Message, done chan bool) {
	if atomic.LoadInt32(&d.shutdown) != 0 {
		if done != nil {
			close(done)
		}
		return
	}
	d.newsChan <- &blockMsg{(msg).(*pb.BlockPb), pb.MsgBlockProtoMsgType, done}
}

// dispatchBlockSyncReq adds the passed block sync request to the news handling queue.
func (d *iotxDispatcher) dispatchBlockSyncReq(sender string, msg proto.Message, done chan bool) {
	if atomic.LoadInt32(&d.shutdown) != 0 {
		if done != nil {
			close(done)
		}
		return
	}
	d.newsChan <- &blockSyncMsg{sender, (msg).(*pb.BlockSync), done}
}

// dispatchBlockSyncData handles block sync data
func (d *iotxDispatcher) dispatchBlockSyncData(msg proto.Message, done chan bool) {
	if atomic.LoadInt32(&d.shutdown) != 0 {
		if done != nil {
			close(done)
		}
		return
	}
	data := (msg).(*pb.BlockContainer)
	d.newsChan <- &blockMsg{data.Block, pb.MsgBlockSyncDataType, done}
}

// dispatchAction adds the passed action message to the news handling queue.
func (d *iotxDispatcher) dispatchAction(msg proto.Message, done chan bool) {
	if atomic.LoadInt32(&d.shutdown) != 0 {
		if done != nil {
			close(done)
		}
		return
	}
	d.newsChan <- &actionMsg{(msg).(*pb.ActionPb), done}
}

// HandleBroadcast handles incoming broadcast message
func (d *iotxDispatcher) HandleBroadcast(message proto.Message, done chan bool) {
	msgType, err := pb.GetTypeFromProtoMsg(message)
	if err != nil {
		logger.Warn().
			Str("error", err.Error()).
			Msg("unexpected message handled by HandleBroadcast")
	}

	switch msgType {
	case pb.ViewChangeMsgType:
		d.cs.HandleViewChange(message, done)
	case pb.MsgTxProtoMsgType:
		d.dispatchTx(message, done)
	case pb.MsgBlockProtoMsgType:
		d.dispatchBlockCommit(message, done)
	case pb.MsgActionType:
		d.dispatchAction(message, done)
	default:
		logger.Warn().
			Uint32("msgType", msgType).
			Msg("unexpected msgType handled by HandleBroadcast")
	}
}

// HandleTell handles incoming unicast message
func (d *iotxDispatcher) HandleTell(sender net.Addr, message proto.Message, done chan bool) {
	msgType, err := pb.GetTypeFromProtoMsg(message)
	if err != nil {
		logger.Warn().
			Str("error", err.Error()).
			Msg("unexpected message handled by HandleTell")
	}

	logger.Info().
		Str("sender", sender.String()).
		Str("message", message.String()).
		Msg("dispatcher.HandleTell from")
	switch msgType {
	case pb.MsgBlockSyncReqType:
		d.dispatchBlockSyncReq(sender.String(), message, done)
	case pb.MsgBlockSyncDataType:
		d.dispatchBlockSyncData(message, done)
	case pb.MsgBlockProtoMsgType:
		d.cs.HandleBlockPropose(message, done)
	default:
		logger.Warn().
			Uint32("msgType", msgType).
			Msg("unexpected msgType handled by HandleTell")
	}
}
