// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"context"
	"math/rand"
	"sync"

	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/routine"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
)

type syncBlocksInterval struct {
	Start uint64
	End   uint64
}

type syncWorker struct {
	chainID          uint32
	mu               sync.RWMutex
	targetHeight     uint64
	unicastHandler   UnicastOutbound
	neighborsHandler Neighbors
	buf              *blockBuffer
	task             *routine.RecurringTask
	maxRepeat        int
	repeatDecayStep  int
}

func newSyncWorker(
	chainID uint32,
	cfg config.Config,
	unicastHandler UnicastOutbound,
	neighborsHandler Neighbors,
	buf *blockBuffer,
) *syncWorker {
	w := &syncWorker{
		chainID:          chainID,
		unicastHandler:   unicastHandler,
		neighborsHandler: neighborsHandler,
		buf:              buf,
		targetHeight:     0,
		maxRepeat:        cfg.BlockSync.MaxRepeat,
		repeatDecayStep:  cfg.BlockSync.RepeatDecayStep,
	}
	if cfg.BlockSync.Interval != 0 {
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

	ctx := context.Background()
	peers, err := w.neighborsHandler(ctx)
	if len(peers) == 0 {
		log.L().Debug("No peer exist to sync with.")
		return
	}
	if err != nil {
		log.L().Warn("Error when get neighbor peers.", zap.Error(err))
		return
	}
	intervals := w.buf.GetBlocksIntervalsToSync(w.targetHeight)
	if intervals != nil {
		log.L().Debug("block sync intervals.",
			zap.Any("intervals", intervals),
			zap.Uint64("targetHeight", w.targetHeight))
	}

	for i, interval := range intervals {
		repeat := w.maxRepeat - i/w.repeatDecayStep
		if repeat <= 0 {
			repeat = 1
		}
		for j := 0; j < repeat; j++ {
			rrIdx := rand.Intn(len(peers))
			p := peers[rrIdx]
			if err := w.unicastHandler(ctx, p, &iotexrpc.BlockSync{
				Start: interval.Start, End: interval.End,
			}); err != nil {
				log.L().Debug("Failed to sync block.", zap.Error(err))
			}
		}
	}
}
