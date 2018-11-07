// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"context"
	"net"
	"sync"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/pkg/routine"
	pb "github.com/iotexproject/iotex-core/proto"
)

type syncBlocksInterval struct {
	Start uint64
	End   uint64
}

type syncWorker struct {
	chainID      uint32
	mu           sync.RWMutex
	targetHeight uint64
	p2p          network.Overlay
	rrIdx        int
	buf          *blockBuffer
	task         *routine.RecurringTask
}

func newSyncWorker(chainID uint32, cfg *config.Config, p2p network.Overlay, buf *blockBuffer) *syncWorker {
	w := &syncWorker{
		chainID:      chainID,
		p2p:          p2p,
		buf:          buf,
		targetHeight: 0,
		rrIdx:        0,
	}
	if interval := syncTaskInterval(cfg); interval != 0 {
		w.task = routine.NewRecurringTask(w.Sync, cfg.BlockSync.Interval)
	}
	return w
}

func (w *syncWorker) Start(ctx context.Context) error {
	if w.task != nil {
		return w.task.Start(ctx)
	}
	return nil
}

func (w *syncWorker) Stop(ctx context.Context) error {
	if w.task != nil {
		return w.task.Stop(ctx)
	}
	return nil
}

func (w *syncWorker) SetTargetHeight(h uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if h > w.targetHeight {
		w.targetHeight = h
	}
}

// Sync checks the sliding window and send more sync request if needed
func (w *syncWorker) Sync() {
	w.mu.Lock()
	defer w.mu.Unlock()

	peers := w.p2p.GetPeers()
	if len(peers) == 0 {
		logger.Debug().Msg("No peer exist to sync with.")
		return
	}
	intervals := w.buf.GetBlocksIntervalsToSync(w.targetHeight)
	if intervals != nil {
		logger.Info().Interface("intervals", intervals).Uint64("targetHeight", w.targetHeight).Msg("block sync intervals.")
	}
	for _, interval := range intervals {
		w.rrIdx = w.rrIdx % len(peers)
		p := peers[w.rrIdx]
		if err := w.sync(p, interval); err != nil {
			logger.Warn().Err(err).Msg("Failed to sync block.")
		}
		w.rrIdx++
	}
}

func (w *syncWorker) sync(p net.Addr, interval syncBlocksInterval) error {
	return w.p2p.Tell(w.chainID, p, &pb.BlockSync{
		Start: interval.Start, End: interval.End,
	})
}
