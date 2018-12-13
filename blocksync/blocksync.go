// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/network/node"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/routine"
	pb "github.com/iotexproject/iotex-core/proto"
)

// BlockSync defines the interface of blocksyncer
type BlockSync interface {
	lifecycle.StartStopper

	TargetHeight() uint64
	P2P() network.Overlay
	ProcessSyncRequest(sender string, sync *pb.BlockSync) error
	ProcessBlock(blk *blockchain.Block) error
	ProcessBlockSync(blk *blockchain.Block) error
}

// blockSyncer implements BlockSync interface
type blockSyncer struct {
	ackBlockCommit bool   // acknowledges latest committed block
	ackBlockSync   bool   // acknowledges old block from sync request
	ackSyncReq     bool   // acknowledges incoming Sync request
	commitHeight   uint64 // last commit block height
	buf            *blockBuffer
	worker         *syncWorker
	bc             blockchain.Blockchain
	p2p            network.Overlay
	chaser         *routine.RecurringTask
}

// NewBlockSyncer returns a new block syncer instance
func NewBlockSyncer(
	cfg config.Config,
	chain blockchain.Blockchain,
	ap actpool.ActPool,
	p2p network.Overlay,
) (BlockSync, error) {
	if chain == nil || ap == nil || p2p == nil {
		return nil, errors.New("cannot create BlockSync: missing param")
	}
	bufSize := cfg.BlockSync.BufferSize
	if cfg.IsFullnode() {
		bufSize <<= 3
	}
	buf := &blockBuffer{
		blocks: make(map[uint64]*blockchain.Block),
		bc:     chain,
		ap:     ap,
		size:   bufSize,
	}
	w := newSyncWorker(chain.ChainID(), cfg, p2p, buf)
	bs := &blockSyncer{
		ackBlockCommit: cfg.IsDelegate() || cfg.IsFullnode(),
		ackBlockSync:   cfg.IsDelegate() || cfg.IsFullnode(),
		ackSyncReq:     cfg.IsDelegate() || cfg.IsFullnode(),
		bc:             chain,
		buf:            buf,
		p2p:            p2p,
		worker:         w,
	}
	bs.chaser = routine.NewRecurringTask(bs.Chase, cfg.BlockSync.Interval*10)
	return bs, nil
}

// TargetHeight returns the target height to sync to
func (bs *blockSyncer) TargetHeight() uint64 {
	return bs.worker.targetHeight
}

// P2P returns the network overlay object
func (bs *blockSyncer) P2P() network.Overlay {
	return bs.p2p
}

// Start starts a block syncer
func (bs *blockSyncer) Start(ctx context.Context) error {
	logger.Debug().Msg("Starting block syncer")
	if err := bs.chaser.Start(ctx); err != nil {
		return err
	}
	if err := bs.worker.Start(ctx); err != nil {
		return err
	}
	bs.commitHeight = bs.buf.CommitHeight()
	return nil
}

// Stop stops a block syncer
func (bs *blockSyncer) Stop(ctx context.Context) error {
	logger.Debug().Msg("Stopping block syncer")
	if err := bs.chaser.Stop(ctx); err != nil {
		return err
	}
	return bs.worker.Stop(ctx)
}

// ProcessBlock processes an incoming latest committed block
func (bs *blockSyncer) ProcessBlock(blk *blockchain.Block) error {
	if !bs.ackBlockCommit {
		// node is not meant to handle latest committed block, simply exit
		return nil
	}

	var needSync bool
	moved, re := bs.buf.Flush(blk)
	switch re {
	case bCheckinLower:
		logger.Debug().Msg("Drop block lower than buffer's accept height.")
	case bCheckinExisting:
		logger.Debug().Msg("Drop block exists in buffer.")
	case bCheckinHigher:
		needSync = true
	case bCheckinValid:
		needSync = !moved
	case bCheckinSkipNil:
		needSync = false
	}

	if needSync {
		bs.worker.SetTargetHeight(blk.Height())
	}
	return nil
}

func (bs *blockSyncer) ProcessBlockSync(blk *blockchain.Block) error {
	if !bs.ackBlockSync {
		// node is not meant to handle sync block, simply exit
		return nil
	}
	bs.buf.Flush(blk)
	if bs.bc.TipHeight() == bs.TargetHeight() {
		bs.worker.SetTargetHeight(bs.TargetHeight() + bs.buf.bufSize())
	}
	return nil
}

// ProcessSyncRequest processes a block sync request
func (bs *blockSyncer) ProcessSyncRequest(sender string, sync *pb.BlockSync) error {
	if !bs.ackSyncReq {
		// node is not meant to handle sync request, simply exit
		return nil
	}

	for i := sync.Start; i <= sync.End; i++ {
		blk, err := bs.bc.GetBlockByHeight(i)
		if err != nil {
			return err
		}
		// TODO: send back multiple blocks in one shot
		if err := bs.p2p.Tell(bs.bc.ChainID(), node.NewTCPNode(sender), &pb.BlockContainer{Block: blk.ConvertToBlockPb()}); err != nil {
			logger.Warn().Err(err).Msg("Failed to response to ProcessSyncRequest.")
		}
	}
	return nil
}

// Chase sets the block sync target height to be blockchain height + 1
func (bs *blockSyncer) Chase() {
	if bs.commitHeight != bs.buf.CommitHeight() {
		bs.commitHeight = bs.buf.CommitHeight()
		return
	}
	// commit height hasn't changed since last chase interval
	bs.worker.SetTargetHeight(bs.bc.TipHeight() + 1)
	logger.Info().Uint64("stuck", bs.commitHeight).Msg("Chaser")
}
