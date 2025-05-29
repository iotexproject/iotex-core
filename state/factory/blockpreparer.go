package factory

import (
	"context"
	"sync"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/prometheustimer"
)

var (
	_blockPreparerLatencyTimer *prometheustimer.TimerFactory
)

type (
	blockPreparer struct {
		tasks   map[hash.Hash256]map[int64]chan struct{}
		results map[hash.Hash256]map[int64]*mintResult
		mu      sync.Mutex
	}
	mintResult struct {
		blk *block.Block
		err error
	}
)

func init() {
	var err error
	_blockPreparerLatencyTimer, err = prometheustimer.New(
		"iotex_factory_block_preparer_latency_nanoseconds",
		"Latency of block preparer operations in nanoseconds",
		[]string{"operation"},
		[]string{"unknown"},
	)
	if err != nil {
		log.L().Error("Failed to create block preparer latency timer", zap.Error(err))
	}
}

func newBlockPreparer() *blockPreparer {
	return &blockPreparer{
		tasks:   make(map[hash.Hash256]map[int64]chan struct{}),
		results: make(map[hash.Hash256]map[int64]*mintResult),
	}
}

func (d *blockPreparer) PrepareOrWait(ctx context.Context, prevHash []byte, timestamp time.Time, fn func() (*block.Block, error)) (*block.Block, error) {
	var (
		preMintTimer = _blockPreparerLatencyTimer.NewTimer("premint")
	)

	d.mu.Lock()
	task, existed := d.getOrNewTask(prevHash, timestamp)
	if !existed {
		go func() {
			preMintTimer.End()
			blk, err := fn()
			d.mu.Lock()
			if _, ok := d.results[hash.BytesToHash256(prevHash)]; !ok {
				d.results[hash.BytesToHash256(prevHash)] = make(map[int64]*mintResult)
			}
			d.results[hash.BytesToHash256(prevHash)][timestamp.UnixNano()] = &mintResult{blk: blk, err: err}
			d.mu.Unlock()
			close(task)
			log.L().Debug("prepare mint returned", zap.Error(err))
		}()
	}
	d.mu.Unlock()

	select {
	case <-task:
	case <-ctx.Done():
		var null *block.Block
		return null, errors.Wrapf(ctx.Err(), "wait for draft block timeout %v", timestamp)
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if blks, ok := d.results[hash.BytesToHash256(prevHash)]; ok && blks[timestamp.UnixNano()] != nil {
		res := blks[timestamp.UnixNano()]
		return res.blk, res.err
	}
	return nil, errors.New("mint result not found")
}

func (d *blockPreparer) getOrNewTask(prevHash []byte, timestamp time.Time) (chan struct{}, bool) {
	if forks, ok := d.tasks[hash.BytesToHash256(prevHash)]; ok {
		if ch, ok := forks[timestamp.UnixNano()]; ok {
			log.L().Debug("draft block already exists", log.Hex("prevHash", prevHash))
			return ch, true
		}
	} else {
		d.tasks[hash.BytesToHash256(prevHash)] = make(map[int64]chan struct{})
	}
	task := make(chan struct{})
	d.tasks[hash.BytesToHash256(prevHash)][timestamp.UnixNano()] = task
	return task, false
}

func (d *blockPreparer) ReceiveBlock(blk *block.Block) error {
	d.mu.Lock()
	delete(d.tasks, blk.PrevHash())
	delete(d.results, blk.PrevHash())
	d.mu.Unlock()
	return nil
}
