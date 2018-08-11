// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package dispatch

import (
	"context"
	"net"
	"sync"
	"sync/atomic"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/dispatch/dispatcher"
	"github.com/iotexproject/iotex-core/logger"
	pb "github.com/iotexproject/iotex-core/proto"
)

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

// IotxDispatcher is the request and event dispatcher for iotx node.
type IotxDispatcher struct {
	started    int32
	shutdown   int32
	eventChan  chan interface{}
	eventAudit map[uint32]int
	wg         sync.WaitGroup
	quit       chan struct{}

	bs blocksync.BlockSync
	cs consensus.Consensus
	ap actpool.ActPool
}

// NewDispatcher creates a new Dispatcher
func NewDispatcher(
	cfg *config.Config,
	ap actpool.ActPool,
	bs blocksync.BlockSync,
	cs consensus.Consensus,
) (dispatcher.Dispatcher, error) {
	if bs == nil {
		return nil, errors.New("Try to attach to a nil P2P")
	}
	d := &IotxDispatcher{
		eventChan:  make(chan interface{}, cfg.Dispatcher.EventChanSize),
		eventAudit: make(map[uint32]int),
		quit:       make(chan struct{}),
		ap:         ap,
		bs:         bs,
		cs:         cs,
	}
	return d, nil
}

// Start starts the dispatcher.
func (d *IotxDispatcher) Start(ctx context.Context) error {
	if atomic.AddInt32(&d.started, 1) != 1 {
		return errors.New("Dispatcher already started")
	}

	logger.Info().Msg("Starting dispatcher")
	if err := d.cs.Start(ctx); err != nil {
		return err
	}

	if err := d.bs.Start(ctx); err != nil {
		return err
	}

	d.wg.Add(1)
	go d.newsHandler()
	return nil
}

// Stop gracefully shuts down the dispatcher by stopping all handlers and waiting for them to finish.
func (d *IotxDispatcher) Stop(ctx context.Context) error {
	if atomic.AddInt32(&d.shutdown, 1) != 1 {
		logger.Warn().Msg("Dispatcher already in the process of shutting down")
		return nil
	}

	logger.Info().Msg("Dispatcher is shutting down")
	if err := d.cs.Stop(ctx); err != nil {
		return err
	}

	if err := d.bs.Stop(ctx); err != nil {
		return err
	}

	close(d.quit)
	d.wg.Wait()
	return nil
}

// EventChan returns the event chan
func (d *IotxDispatcher) EventChan() *chan interface{} {
	return &d.eventChan
}

// EventAudit returns the event audit map
func (d *IotxDispatcher) EventAudit() map[uint32]int {
	return d.eventAudit
}

// newsHandler is the main handler for handling all news from peers.
func (d *IotxDispatcher) newsHandler() {
loop:
	for {
		select {
		case m := <-d.eventChan:
			switch msg := m.(type) {
			case *actionMsg:
				d.handleActionMsg(msg)
			case *blockMsg:

				d.handleBlockMsg(msg)
			case *blockSyncMsg:
				d.handleBlockSyncMsg(msg)

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

// handleActionMsg handles actionMsg from all peers.
func (d *IotxDispatcher) handleActionMsg(m *actionMsg) {
	d.eventAudit[pb.MsgActionType] = d.eventAudit[pb.MsgActionType] + 1
	if pbTsf := m.action.GetTransfer(); pbTsf != nil {
		tsf := &action.Transfer{}
		tsf.ConvertFromActionPb(m.action)
		if err := d.ap.AddTsf(tsf); err != nil {
			logger.Error().Err(err)
		}
	} else if pbVote := m.action.GetVote(); pbVote != nil {
		vote := &action.Vote{}
		vote.ConvertFromActionPb(m.action)
		if err := d.ap.AddVote(vote); err != nil {
			logger.Error().Err(err)
		}
	}
	// signal to let caller know we are done
	if m.done != nil {
		m.done <- true
	}
}

// handleBlockMsg handles blockMsg from peers.
func (d *IotxDispatcher) handleBlockMsg(m *blockMsg) {
	blk := &blockchain.Block{}
	blk.ConvertFromBlockPb(m.block)
	hash := blk.HashBlock()
	logger.Info().
		Uint64("block", blk.Height()).Hex("hash", hash[:]).Msg("receive blockMsg")

	if m.blkType == pb.MsgBlockProtoMsgType {
		d.eventAudit[pb.MsgBlockProtoMsgType] = d.eventAudit[pb.MsgBlockProtoMsgType] + 1
		if err := d.bs.ProcessBlock(blk); err != nil {
			logger.Error().Err(err).Msg("Fail to process the block")
		}
	} else if m.blkType == pb.MsgBlockSyncDataType {
		d.eventAudit[pb.MsgBlockSyncDataType] = d.eventAudit[pb.MsgBlockSyncDataType] + 1
		if err := d.bs.ProcessBlockSync(blk); err != nil {
			logger.Error().Err(err).Msg("Fail to sync the block")
		}
	}
	// signal to let caller know we are done
	if m.done != nil {
		m.done <- true
	}
}

// handleBlockSyncMsg handles block messages from peers.
func (d *IotxDispatcher) handleBlockSyncMsg(m *blockSyncMsg) {
	logger.Info().
		Str("src", m.sender).
		Uint64("start", m.sync.Start).
		Uint64("end", m.sync.End).
		Msg("receive blockSyncMsg")
	d.eventAudit[pb.MsgBlockSyncReqType] = d.eventAudit[pb.MsgBlockSyncReqType] + 1
	// dispatch to block sync
	if err := d.bs.ProcessSyncRequest(m.sender, m.sync); err != nil {
		logger.Error().Err(err)
	}
	// signal to let caller know we are done
	if m.done != nil {
		m.done <- true
	}
}

// dispatchAction adds the passed action message to the news handling queue.
func (d *IotxDispatcher) dispatchAction(msg proto.Message, done chan bool) {
	if atomic.LoadInt32(&d.shutdown) != 0 {
		if done != nil {
			close(done)
		}
		return
	}
	d.enqueueEvent(&actionMsg{(msg).(*pb.ActionPb), done})
}

// dispatchBlockCommit adds the passed block message to the news handling queue.
func (d *IotxDispatcher) dispatchBlockCommit(msg proto.Message, done chan bool) {
	if atomic.LoadInt32(&d.shutdown) != 0 {
		if done != nil {
			close(done)
		}
		return
	}
	d.enqueueEvent(&blockMsg{(msg).(*pb.BlockPb), pb.MsgBlockProtoMsgType, done})
}

// dispatchBlockSyncReq adds the passed block sync request to the news handling queue.
func (d *IotxDispatcher) dispatchBlockSyncReq(sender string, msg proto.Message, done chan bool) {
	if atomic.LoadInt32(&d.shutdown) != 0 {
		if done != nil {
			close(done)
		}
		return
	}
	d.enqueueEvent(&blockSyncMsg{sender, (msg).(*pb.BlockSync), done})
}

// dispatchBlockSyncData handles block sync data
func (d *IotxDispatcher) dispatchBlockSyncData(msg proto.Message, done chan bool) {
	if atomic.LoadInt32(&d.shutdown) != 0 {
		if done != nil {
			close(done)
		}
		return
	}
	data := (msg).(*pb.BlockContainer)
	d.enqueueEvent(&blockMsg{data.Block, pb.MsgBlockSyncDataType, done})
}

// HandleBroadcast handles incoming broadcast message
func (d *IotxDispatcher) HandleBroadcast(message proto.Message, done chan bool) {
	msgType, err := pb.GetTypeFromProtoMsg(message)
	if err != nil {
		logger.Warn().
			Str("error", err.Error()).
			Msg("unexpected message handled by HandleBroadcast")
	}

	switch msgType {
	case pb.ViewChangeMsgType:
		err := d.cs.HandleViewChange(message, done)
		if err != nil {
			logger.Error().
				Err(err).
				Msgf("failed to handle view change")
		}
	case pb.MsgActionType:
		d.dispatchAction(message, done)
	case pb.MsgBlockProtoMsgType:
		d.dispatchBlockCommit(message, done)
	default:
		logger.Warn().
			Uint32("msgType", msgType).
			Msg("unexpected msgType handled by HandleBroadcast")
	}
}

// HandleTell handles incoming unicast message
func (d *IotxDispatcher) HandleTell(sender net.Addr, message proto.Message, done chan bool) {
	msgType, err := pb.GetTypeFromProtoMsg(message)
	if err != nil {
		logger.Warn().
			Str("error", err.Error()).
			Msg("unexpected message handled by HandleTell")
	}

	switch msgType {
	case pb.MsgBlockSyncReqType:
		d.dispatchBlockSyncReq(sender.String(), message, done)
	case pb.MsgBlockSyncDataType:
		d.dispatchBlockSyncData(message, done)
	case pb.MsgBlockProtoMsgType:
		err := d.cs.HandleBlockPropose(message, done)
		if err != nil {
			logger.Error().
				Err(err).
				Msgf("failed to handle block propose")
		}
	default:
		logger.Warn().
			Uint32("msgType", msgType).
			Msg("unexpected msgType handled by HandleTell")
	}
}

func (d *IotxDispatcher) enqueueEvent(event interface{}) {
	go func() {
		if len(d.eventChan) == cap(d.eventChan) {
			logger.Warn().Msg("dispatcher event chan is full, drop an event")
			return
		}
		d.eventChan <- event
	}()
}
