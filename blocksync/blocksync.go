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
	unicastCB   Unicast
	neighborsCB Neighbors
}

// Option is the option to override the blocksync config
type Option func(cfg *Config) error

// WithUnicast is the option to set the unicast callback
func WithUnicast(unicastCB Unicast) Option {
	return func(cfg *Config) error {
		cfg.unicastCB = unicastCB
		return nil
	}
}

// WithNeighbors is the option to set the neighbors callback
func WithNeighbors(neighborsCB Neighbors) Option {
	return func(cfg *Config) error {
		cfg.neighborsCB = neighborsCB
		return nil
	}
}

// BlockSync defines the interface of blocksyncer
type BlockSync interface {
	lifecycle.StartStopper

	TargetHeight() uint64
	ProcessSyncRequest(sender string, sync *iproto.BlockSync) error
	ProcessBlock(blk *blockchain.Block) error
	ProcessBlockSync(blk *blockchain.Block) error
}

// blockSyncer implements BlockSync interface
type blockSyncer struct {
	ackBlockCommit bool // acknowledges latest committed block
	ackBlockSync   bool // acknowledges old block from sync request
	ackSyncReq     bool // acknowledges incoming Sync request
	buf            *blockBuffer
	worker         *syncWorker
	bc             blockchain.Blockchain
	unicastCB      Unicast
	neighborsCB    Neighbors
	chaser         *routine.RecurringTask
}

// NewBlockSyncer returns a new block syncer instance
func NewBlockSyncer(
	cfg config.Config,
	chain blockchain.Blockchain,
	ap actpool.ActPool,
	opts ...Option,
) (BlockSync, error) {
	buf := &blockBuffer{
		blocks: make(map[uint64]*blockchain.Block),
		bc:     chain,
		ap:     ap,
		size:   cfg.BlockSync.BufferSize,
	}
	bsCfg := Config{}
	for _, opt := range opts {
		opt(&bsCfg)
	}
	bs := &blockSyncer{
		ackBlockCommit: cfg.IsDelegate() || cfg.IsFullnode(),
		ackBlockSync:   cfg.IsDelegate() || cfg.IsFullnode(),
		ackSyncReq:     cfg.IsDelegate() || cfg.IsFullnode(),
		bc:             chain,
		buf:            buf,
		unicastCB:      bsCfg.unicastCB,
		neighborsCB:    bsCfg.neighborsCB,
		worker:         newSyncWorker(chain.ChainID(), cfg, bsCfg.unicastCB, bsCfg.neighborsCB, buf),
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
	return bs.worker.Start(ctx)
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
		if err := bs.unicastCB(
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
	bs.worker.SetTargetHeight(bs.bc.TipHeight() + 1)
}
