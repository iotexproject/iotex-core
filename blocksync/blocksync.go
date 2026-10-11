// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/v2/nodeinfo"
	"github.com/iotexproject/iotex-core/v2/pkg/fastrand"
	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/routine"
	"github.com/iotexproject/iotex-core/v2/server/itx/nodestats"
)

type (
	// Neighbors acquires p2p neighbors in the network
	Neighbors func() ([]peer.AddrInfo, error)
	// UniCastOutbound sends a unicase message to the peer
	UniCastOutbound func(context.Context, peer.AddrInfo, proto.Message) error
	// BlockPeer adds the peer into blacklist in p2p layer
	BlockPeer func(string)
	// TipHeight returns the tip height of blockchain
	TipHeight func() uint64
	// BlockByHeight returns the block of a given height
	BlockByHeight func(uint64) (*block.Block, error)
	// CommitBlock commits a block to blockchain
	CommitBlock func(*block.Block) error

	// BlockSync defines the interface of blocksyncer
	BlockSync interface {
		lifecycle.StartStopper
		nodestats.StatsReporter
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
		cfg Config
		buf *blockBuffer

		tipHeightHandler     TipHeight
		blockByHeightHandler BlockByHeight
		commitBlockHandler   CommitBlock
		p2pNeighbor          Neighbors
		unicastOutbound      UniCastOutbound
		blockP2pPeer         BlockPeer
		nodeInfoManager      *nodeinfo.InfoManager

		syncTask      *routine.RecurringTask
		syncStageTask *routine.RecurringTask

		syncStageHeight   uint64
		syncBlockIncrease uint64

		startingHeight uint64 // block number this node started to synchronise from
		lastTip        uint64
		lastTipTime    time.Time
		targetHeight   uint64 // block number of the highest block header this node has received from peers
		mu             sync.RWMutex
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

func (*dummyBlockSync) BuildReport() string {
	return ""
}

// NewBlockSyncer returns a new block syncer instance
func NewBlockSyncer(
	cfg Config,
	tipHeightHandler TipHeight,
	blockByHeightHandler BlockByHeight,
	commitBlockHandler CommitBlock,
	p2pNeighbor Neighbors,
	uniCastHandler UniCastOutbound,
	blockP2pPeer BlockPeer,
	nodeInfoManager *nodeinfo.InfoManager,
) (BlockSync, error) {
	bs := &blockSyncer{
		cfg:                  cfg,
		lastTipTime:          time.Time{},
		buf:                  newBlockBuffer(cfg.BufferSize, cfg.IntervalSize),
		tipHeightHandler:     tipHeightHandler,
		blockByHeightHandler: blockByHeightHandler,
		commitBlockHandler:   commitBlockHandler,
		p2pNeighbor:          p2pNeighbor,
		unicastOutbound:      uniCastHandler,
		blockP2pPeer:         blockP2pPeer,
		targetHeight:         0,
		nodeInfoManager:      nodeInfoManager,
	}
	if bs.cfg.Interval != 0 {
		bs.syncTask = routine.NewRecurringTask(bs.sync, bs.cfg.Interval)
		bs.syncStageTask = routine.NewRecurringTask(bs.syncStageChecker, bs.cfg.Interval)
	}
	atomic.StoreUint64(&bs.syncBlockIncrease, 0)
	return bs, nil
}

// commitBlocks commits the first candidate that is accepted. It reports whether
// one was committed, and whether any candidate was rejected as invalid (its
// peer is blocked).
func (bs *blockSyncer) commitBlocks(blks []*peerBlock) (committed bool, rejected bool) {
	for _, blk := range blks {
		if blk == nil {
			continue
		}
		err := bs.commitBlockHandler(blk.block)
		switch errors.Cause(err) {
		case nil:
			if blk.block.Height() > bs.lastTip {
				bs.lastTip = blk.block.Height()
				bs.lastTipTime = blk.block.Timestamp()
			}
			return true, rejected
		case blockdao.ErrRemoteHeightTooLow:
			log.L().Info("remote height too low", zap.Uint64("height", blk.block.Height()))
		case blockchain.ErrPaused:
			log.L().Info("blockchain is paused, skip committing block", zap.Uint64("height", blk.block.Height()))
		default:
			rejected = true
			bs.blockP2pPeer(blk.pid)
			log.L().Error("failed to commit block", zap.Error(err), zap.Uint64("height", blk.block.Height()), zap.String("peer", blk.pid))
		}
	}
	return false, rejected
}

func (bs *blockSyncer) flushInfo() (time.Time, uint64) {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	return bs.lastTipTime, bs.targetHeight
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
	peers, err := bs.p2pNeighbor()
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
		var peer *peer.AddrInfo
		for j := 0; j < 10; j++ {
			peer = &peers[fastrand.Uint32n(uint32(len(peers)))]
			if bs.nodeInfoManager.MayHaveBlock(peer.ID.String(), start) {
				break
			}
		}
		if peer == nil {
			continue
		}
		if err := bs.unicastOutbound(
			ctx,
			*peer,
			&iotexrpc.BlockSync{Start: start, End: end},
		); err != nil {
			log.L().Error("failed to request blocks", zap.Error(err), zap.String("peer", peer.ID.String()), zap.Uint64("start", start), zap.Uint64("end", end))
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

	if height := bs.processBlock(peer, blk); height > 0 {
		// Every candidate buffered for this height was invalid, and the height
		// had refused or evicted candidates for lack of room, so the authentic
		// block may have been among those lost. Ask for it again now instead of
		// leaving the height to stall; the peers that served the invalid
		// candidates were blocked in commitBlocks.
		log.L().Info("re-requesting height whose buffered candidates all failed", zap.Uint64("height", height))
		bs.requestBlock(ctx, height, height, bs.cfg.MaxRepeat)
	}
	return nil
}

// processBlock buffers blk and commits as far as the buffer allows. It returns
// a height that has to be re-requested, or 0.
func (bs *blockSyncer) processBlock(peer string, blk *block.Block) uint64 {
	tip := bs.tipHeightHandler()
	added, targetHeight := bs.buf.AddBlock(tip, newPeerBlock(peer, blk))
	bs.mu.Lock()
	defer bs.mu.Unlock()
	if targetHeight > bs.targetHeight {
		bs.targetHeight = targetHeight
	}
	if !added {
		return 0
	}
	var (
		syncedHeight = tip
		retry        uint64
	)
	for {
		next := syncedHeight + 1
		blks, lossy := bs.buf.Pop(next)
		committed, rejected := bs.commitBlocks(blks)
		if !committed {
			// Recovery fires at most once per height: only a height at or above
			// tip+2 can fill up and turn lossy (a block at tip+1 is committed on
			// arrival), and the tip never moves back below it. Each occurrence
			// also needs a full queue of rejected candidates, i.e. at least
			// maxBlocksPerHeight/maxBlocksPerPeerPerHeight peers, all of which
			// commitBlocks has just blocked.
			if lossy && rejected {
				retry = next
			}
			break
		}
		syncedHeight++
	}
	log.L().Debug("flush blocks", zap.Uint64("start", tip), zap.Uint64("end", syncedHeight))
	return retry
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
	if start > end {
		return nil
	}
	// Bound the egress a single request can force. GetBlocksIntervalsToSync
	// never asks for more than the sync window, so an honest requester is
	// unaffected; without the bound one small request makes this node read and
	// unicast the entire chain while holding a blockSync worker.
	if window := bs.cfg.BufferSize; window > 0 && end-start >= window {
		end = start + window - 1
		log.L().Debug(
			"Clamping sync request range",
			zap.Uint64("start", start),
			zap.Uint64("end", end),
			zap.Uint64("window", window),
		)
	}
	// TODO: send back multiple blocks in one shot
	for i := start; i <= end; i++ {
		// TODO: fetch block from buffer
		blk, err := bs.blockByHeightHandler(i)
		if err != nil {
			return err
		}
		if err := bs.sendBlock(ctx, peer, blk); err != nil {
			return err
		}
	}
	return nil
}

// sendBlock unicasts one block. The timeout is scoped to this call rather than
// deferred inside the caller's loop, where cancel funcs and their timers would
// accumulate for the whole requested range.
func (bs *blockSyncer) sendBlock(ctx context.Context, peer peer.AddrInfo, blk *block.Block) error {
	syncCtx, cancel := context.WithTimeout(ctx, bs.cfg.ProcessSyncRequestTTL)
	defer cancel()
	return bs.unicastOutbound(syncCtx, peer, blk.ConvertToBlockPb())
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

// BuildReport builds a report of block syncer
func (bs *blockSyncer) BuildReport() string {
	startingHeight, tipHeight, targetHeight, syncSpeedDesc := bs.SyncStatus()
	return fmt.Sprintf(
		"BlockSync startingHeight: %d, tipHeight: %d, targetHeight: %d, %s",
		startingHeight,
		tipHeight,
		targetHeight,
		syncSpeedDesc,
	)
}
