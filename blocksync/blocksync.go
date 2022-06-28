// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/routine"
)

type (
	// P2pNeighbor acquires p2p neighbors in the network
	P2pNeighbor func(context.Context) ([]peer.AddrInfo, error)
	// UniCastOutbound sends a unicase message to the peer
	UniCastOutbound func(context.Context, peer.AddrInfo, proto.Message) error
	// BlockP2pPeer adds the peer into blacklist in p2p layer
	BlockP2pPeer func(string)
	// TipHeight returns the tip height of blockchain
	TipHeight func() uint64
	// BlockByHeight returns the block of a given height
	BlockByHeight func(uint64) (*block.Block, error)
	// CommitBlock commits a block to blockchain
	CommitBlock func(*block.Block) error

	// BlockSync defines the interface of blocksyncer
	BlockSync interface {
		lifecycle.StartStopper

		// TargetHeight returns the target height to sync to
		TargetHeight() uint64
		// ProcessSyncRequest processes a block sync request
		ProcessSyncRequest(context.Context, peer.AddrInfo, uint64, uint64) error
		// ProcessBlock processes an incoming block
		ProcessBlock(context.Context, string, *block.Block) error
		// SyncStatus report block sync status
		SyncStatus() (startingHeight uint64, currentHeight uint64, targetHeight uint64, syncSpeedDesc string)
	}

	dummyBlockSync struct{}

	// blockSyncer implements BlockSync interface
	blockSyncer struct {
		cfg config.BlockSync
		buf *blockBuffer

		tipHeightHandler     TipHeight
		blockByHeightHandler BlockByHeight
		commitBlockHandler   CommitBlock
		p2pNeighbor          P2pNeighbor
		unicastOutbound      UniCastOutbound
		blockP2pPeer         BlockP2pPeer

		syncTask      *routine.RecurringTask
		syncStageTask *routine.RecurringTask

		syncStageHeight   uint64
		syncBlockIncrease uint64

		startingHeight    uint64 // block number this node started to synchronise from
		lastTip           uint64
		lastTipUpdateTime time.Time
		targetHeight      uint64 // block number of the highest block header this node has received from peers
		mu                sync.RWMutex
	}

	peerBlock struct {
		pid   string
		block *block.Block
	}
)

func newPeerBlock(pid string, blk *block.Block) *peerBlock {
	return &peerBlock{
		pid:   pid,
		block: blk,
	}
}

// NewDummyBlockSyncer creates a dummy BlockSync
func NewDummyBlockSyncer() BlockSync {
	return &dummyBlockSync{}
}

func (*dummyBlockSync) Start(context.Context) error {
	return nil
}

func (*dummyBlockSync) Stop(context.Context) error {
	return nil
}

func (*dummyBlockSync) TargetHeight() uint64 {
	return 0
}

func (*dummyBlockSync) ProcessSyncRequest(context.Context, peer.AddrInfo, uint64, uint64) error {
	return nil
}

func (*dummyBlockSync) ProcessBlock(context.Context, string, *block.Block) error {
	return nil
}

func (*dummyBlockSync) SyncStatus() (uint64, uint64, uint64, string) {
	return 0, 0, 0, ""
}

// NewBlockSyncer returns a new block syncer instance
func NewBlockSyncer(
	cfg config.BlockSync,
	tipHeightHandler TipHeight,
	blockByHeightHandler BlockByHeight,
	commitBlockHandler CommitBlock,
	p2pNeighbor P2pNeighbor,
	uniCastHandler UniCastOutbound,
	blockP2pPeer BlockP2pPeer,
) (BlockSync, error) {
	bs := &blockSyncer{
		cfg:                  cfg,
		lastTipUpdateTime:    time.Now(),
		buf:                  newBlockBuffer(cfg.BufferSize, cfg.IntervalSize),
		tipHeightHandler:     tipHeightHandler,
		blockByHeightHandler: blockByHeightHandler,
		commitBlockHandler:   commitBlockHandler,
		p2pNeighbor:          p2pNeighbor,
		unicastOutbound:      uniCastHandler,
		blockP2pPeer:         blockP2pPeer,
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
	for _, blk := range blks {
		if blk == nil {
			continue
		}
		err := bs.commitBlockHandler(blk.block)
		if err == nil {
			return true
		}
		bs.blockP2pPeer(blk.pid)
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
	// no sync
	if len(intervals) == 0 {
		return
	}
	// start syncing
	bs.startingHeight = bs.tipHeightHandler()
	log.L().Info("block sync intervals.",
		zap.Any("intervals", intervals),
		zap.Uint64("targetHeight", targetHeight))
	for i, interval := range intervals {
		bs.requestBlock(context.Background(), interval.Start, interval.End, bs.cfg.MaxRepeat-i/bs.cfg.RepeatDecayStep)
	}
}

func (bs *blockSyncer) requestBlock(ctx context.Context, start uint64, end uint64, repeat int) {
	peers, err := bs.p2pNeighbor(ctx)
	if err != nil {
		log.L().Error("failed to get neighbours", zap.Error(err))
		return
	}
	if len(peers) == 0 {
		log.L().Error("no peers")
		return
	}
	if repeat < 2 {
		repeat = 2
	}
	if repeat > len(peers) {
		repeat = len(peers)
	}
	for i := 0; i < repeat; i++ {
		peer := peers[rand.Intn(len(peers))]
		if err := bs.unicastOutbound(
			ctx,
			peer,
			&iotexrpc.BlockSync{Start: start, End: end},
		); err != nil {
			log.L().Error("failed to request blocks", zap.Error(err), zap.String("peer", peer.ID.Pretty()), zap.Uint64("start", start), zap.Uint64("end", end))
		}
	}
}

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

func (bs *blockSyncer) ProcessBlock(ctx context.Context, peer string, blk *block.Block) error {
	if blk == nil {
		return errors.New("block is nil")
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
		if !bs.commitBlocks(bs.buf.Pop(syncedHeight + 1)) {
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

func (bs *blockSyncer) ProcessSyncRequest(ctx context.Context, peer peer.AddrInfo, start uint64, end uint64) error {
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
		if err := bs.unicastOutbound(syncCtx, peer, blk.ConvertToBlockPb()); err != nil {
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

func (bs *blockSyncer) SyncStatus() (uint64, uint64, uint64, string) {
	var syncSpeedDesc string
	syncBlockIncrease := atomic.LoadUint64(&bs.syncBlockIncrease)
	switch {
	case syncBlockIncrease == 1:
		syncSpeedDesc = "synced to blockchain tip"
	case bs.cfg.Interval == 0:
		syncSpeedDesc = "no sync task"
	default:
		syncSpeedDesc = fmt.Sprintf("sync in progress at %.1f blocks/sec", float64(syncBlockIncrease)/bs.cfg.Interval.Seconds())
	}
	return bs.startingHeight, bs.tipHeightHandler(), bs.targetHeight, syncSpeedDesc
}
