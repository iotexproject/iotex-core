// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/routine"
)

type (
	// RequestBlocks send a block request to peers
	RequestBlocks func(ctx context.Context, start uint64, end uint64, repeat int)
	// TipHeight returns the tip height of blockchain
	TipHeight func() uint64
	// BlockByHeight returns the block of a given height
	BlockByHeight func(uint64) (*block.Block, error)
	// CommitBlock commits a block to blockchain
	CommitBlock func(*block.Block) error
)

// BlockSync defines the interface of blocksyncer
type BlockSync interface {
	lifecycle.StartStopper

	TargetHeight() uint64
	ProcessSyncRequest(context.Context, uint64, uint64, func(context.Context, *block.Block) error) error
	ProcessBlock(context.Context, string, *block.Block) error
	SyncStatus() string
}

// blockSyncer implements BlockSync interface
type blockSyncer struct {
	cfg config.BlockSync
	buf *blockBuffer

	tipHeightHandler     TipHeight
	blockByHeightHandler BlockByHeight
	commitBlockHandler   CommitBlock
	requestBlocksHandler RequestBlocks

	syncTask      *routine.RecurringTask
	syncStageTask *routine.RecurringTask

	syncStageHeight   uint64
	syncBlockIncrease uint64

	lastTip           uint64
	lastTipUpdateTime time.Time
	targetHeight      uint64
	mu                sync.RWMutex

	peerBlockList sync.Map
}

type peerBlock struct {
	pid   string
	block *block.Block
}

func newPeerBlock(pid string, blk *block.Block) *peerBlock {
	return &peerBlock{
		pid:   pid,
		block: blk,
	}
}

// NewBlockSyncer returns a new block syncer instance
func NewBlockSyncer(
	cfg config.BlockSync,
	tipHeightHandler TipHeight,
	blockByHeightHandler BlockByHeight,
	commitBlockHandler CommitBlock,
	requestBlocksHandler RequestBlocks,
) (BlockSync, error) {
	bs := &blockSyncer{
		cfg:                  cfg,
		lastTipUpdateTime:    time.Now(),
		buf:                  newBlockBuffer(cfg.BufferSize, cfg.IntervalSize),
		tipHeightHandler:     tipHeightHandler,
		blockByHeightHandler: blockByHeightHandler,
		commitBlockHandler:   commitBlockHandler,
		requestBlocksHandler: requestBlocksHandler,
		targetHeight:         0,
	}
	if bs.cfg.Interval != 0 {
		bs.syncTask = routine.NewRecurringTask(bs.sync, bs.cfg.Interval)
		bs.syncStageTask = routine.NewRecurringTask(bs.syncStageChecker, bs.cfg.Interval)
	}
	atomic.StoreUint64(&bs.syncBlockIncrease, 0)
	return bs, nil
}

func (bs *blockSyncer) commitBlocks(blks []*peerBlock) bool {
	if blks == nil {
		return false
	}
	for _, blk := range blks {
		if blk == nil {
			continue
		}
		err := bs.commitBlockHandler(blk.block)
		if err == nil {
			return true
		}
		bs.peerBlockList.Store(blk.pid, true)

		log.L().Error("failed to commit block", zap.Error(err), zap.Uint64("height", blk.block.Height()), zap.String("peer", blk.pid))
	}
	return false
}

func (bs *blockSyncer) flushInfo() (time.Time, uint64) {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	return bs.lastTipUpdateTime, bs.targetHeight
}

func (bs *blockSyncer) sync() {
	updateTime, targetHeight := bs.flushInfo()
	if updateTime.Add(bs.cfg.Interval).After(time.Now()) {
		return
	}
	intervals := bs.buf.GetBlocksIntervalsToSync(bs.tipHeightHandler(), targetHeight)
	if intervals != nil {
		log.L().Info("block sync intervals.",
			zap.Any("intervals", intervals),
			zap.Uint64("targetHeight", targetHeight))
	}

	for i, interval := range intervals {
		bs.requestBlocksHandler(context.Background(), interval.Start, interval.End, bs.cfg.MaxRepeat-i/bs.cfg.RepeatDecayStep)
	}
}

// TargetHeight returns the target height to sync to
func (bs *blockSyncer) TargetHeight() uint64 {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	return bs.targetHeight
}

// Start starts a block syncer
func (bs *blockSyncer) Start(ctx context.Context) error {
	log.L().Debug("Starting block syncer.")
	if bs.syncTask != nil {
		if err := bs.syncTask.Start(ctx); err != nil {
			return err
		}
	}
	if bs.syncStageTask != nil {
		return bs.syncStageTask.Start(ctx)
	}
	return nil
}

// Stop stops a block syncer
func (bs *blockSyncer) Stop(ctx context.Context) error {
	log.L().Debug("Stopping block syncer.")
	if bs.syncStageTask != nil {
		if err := bs.syncStageTask.Stop(ctx); err != nil {
			return err
		}
	}
	if bs.syncTask != nil {
		if err := bs.syncTask.Stop(ctx); err != nil {
			return err
		}
	}
	return nil
}

// ProcessBlock processes an incoming block
func (bs *blockSyncer) ProcessBlock(ctx context.Context, peer string, blk *block.Block) error {
	if blk == nil {
		return errors.New("block is nil")
	}

	_, ok := bs.peerBlockList.Load(peer)
	if ok {
		log.L().Info("peer in block list.")
		return nil
	}

	tip := bs.tipHeightHandler()
	added, targetHeight := bs.buf.AddBlock(tip, newPeerBlock(peer, blk))
	bs.mu.Lock()
	defer bs.mu.Unlock()
	if targetHeight > bs.targetHeight {
		bs.targetHeight = targetHeight
	}
	if !added {
		return nil
	}
	syncedHeight := tip
	for {
		if !bs.commitBlocks(bs.buf.Delete(syncedHeight + 1)) {
			break
		}
		syncedHeight++
	}
	bs.buf.Cleanup(syncedHeight)
	log.L().Debug("flush blocks", zap.Uint64("start", tip), zap.Uint64("end", syncedHeight))
	if syncedHeight > bs.lastTip {
		bs.lastTip = syncedHeight
		bs.lastTipUpdateTime = time.Now()
	}
	return nil
}

// ProcessSyncRequest processes a block sync request
func (bs *blockSyncer) ProcessSyncRequest(ctx context.Context, start uint64, end uint64, callback func(context.Context, *block.Block) error) error {
	tip := bs.tipHeightHandler()
	if end > tip {
		log.L().Debug(
			"Do not have requested blocks",
			zap.Uint64("start", start),
			zap.Uint64("end", end),
			zap.Uint64("tipHeight", tip),
		)
		end = tip
	}
	for i := start; i <= end; i++ {
		// TODO: fetch block from buffer
		blk, err := bs.blockByHeightHandler(i)
		if err != nil {
			return err
		}
		// TODO: send back multiple blocks in one shot
		syncCtx, cancel := context.WithTimeout(ctx, bs.cfg.ProcessSyncRequestTTL)
		defer cancel()
		if err := callback(syncCtx, blk); err != nil {
			return err
		}
	}
	return nil
}

func (bs *blockSyncer) syncStageChecker() {
	tipHeight := bs.tipHeightHandler()
	atomic.StoreUint64(&bs.syncBlockIncrease, tipHeight-bs.syncStageHeight)
	bs.syncStageHeight = tipHeight
}

// SyncStatus report block sync status
func (bs *blockSyncer) SyncStatus() string {
	syncBlockIncrease := atomic.LoadUint64(&bs.syncBlockIncrease)
	if syncBlockIncrease == 1 {
		return "synced to blockchain tip"
	}
	if bs.cfg.Interval == 0 {
		return "no sync task"
	}
	return fmt.Sprintf("sync in progress at %.1f blocks/sec", float64(syncBlockIncrease)/bs.cfg.Interval.Seconds())
}
