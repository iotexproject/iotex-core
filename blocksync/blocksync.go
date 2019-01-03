// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"context"
	"net"

	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/p2p/node"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/routine"
	"github.com/iotexproject/iotex-core/proto"
)

type (
	// Unicast sends a unicast message to the given address
	Unicast func(addr net.Addr, msg proto.Message) error
	// Neighbors returns the neighbors' addresses
	Neighbors func() []net.Addr
)

// Config represents the config to setup blocksync
type Config struct {
	unicastHandler   Unicast
	neighborsHandler Neighbors
}

// Option is the option to override the blocksync config
type Option func(cfg *Config) error

// WithUnicast is the option to set the unicast callback
func WithUnicast(unicastHandler Unicast) Option {
	return func(cfg *Config) error {
		cfg.unicastHandler = unicastHandler
		return nil
	}
}

// WithNeighbors is the option to set the neighbors callback
func WithNeighbors(neighborsHandler Neighbors) Option {
	return func(cfg *Config) error {
		cfg.neighborsHandler = neighborsHandler
		return nil
	}
}

// BlockSync defines the interface of blocksyncer
type BlockSync interface {
	lifecycle.StartStopper

	TargetHeight() uint64
	ProcessSyncRequest(sender string, sync *iproto.BlockSync) error
	ProcessBlock(blk *block.Block) error
	ProcessBlockSync(blk *block.Block) error
}

// blockSyncer implements BlockSync interface
type blockSyncer struct {
	ackBlockCommit   bool   // acknowledges latest committed block
	ackBlockSync     bool   // acknowledges old block from sync request
	ackSyncReq       bool   // acknowledges incoming Sync request
	commitHeight     uint64 // last commit block height
	buf              *blockBuffer
	worker           *syncWorker
	bc               blockchain.Blockchain
	unicastHandler   Unicast
	neighborsHandler Neighbors
	chaser           *routine.RecurringTask
}

// NewBlockSyncer returns a new block syncer instance
func NewBlockSyncer(
	cfg config.Config,
	chain blockchain.Blockchain,
	ap actpool.ActPool,
	opts ...Option,
) (BlockSync, error) {
	bufSize := cfg.BlockSync.BufferSize
	if cfg.IsFullnode() {
		bufSize <<= 3
	}
	buf := &blockBuffer{
		blocks: make(map[uint64]*block.Block),
		bc:     chain,
		ap:     ap,
		size:   bufSize,
	}
	bsCfg := Config{}
	for _, opt := range opts {
		if err := opt(&bsCfg); err != nil {
			return nil, err
		}
	}
	bs := &blockSyncer{
		ackBlockCommit:   cfg.IsDelegate() || cfg.IsFullnode(),
		ackBlockSync:     cfg.IsDelegate() || cfg.IsFullnode(),
		ackSyncReq:       cfg.IsDelegate() || cfg.IsFullnode(),
		bc:               chain,
		buf:              buf,
		unicastHandler:   bsCfg.unicastHandler,
		neighborsHandler: bsCfg.neighborsHandler,
		worker:           newSyncWorker(chain.ChainID(), cfg, bsCfg.unicastHandler, bsCfg.neighborsHandler, buf),
	}
	bs.chaser = routine.NewRecurringTask(bs.Chase, cfg.BlockSync.Interval*10)
	return bs, nil
}

// TargetHeight returns the target height to sync to
func (bs *blockSyncer) TargetHeight() uint64 {
	return bs.worker.targetHeight
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
func (bs *blockSyncer) ProcessBlock(blk *block.Block) error {
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

func (bs *blockSyncer) ProcessBlockSync(blk *block.Block) error {
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
func (bs *blockSyncer) ProcessSyncRequest(sender string, sync *iproto.BlockSync) error {
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
		if err := bs.unicastHandler(
			node.NewTCPNode(sender),
			&iproto.BlockContainer{Block: blk.ConvertToBlockPb()},
		); err != nil {
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
