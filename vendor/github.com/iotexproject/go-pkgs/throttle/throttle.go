package throttle

import (
	"context"
	"sync"
	"time"
)

const (
	_defaultNumWorkers = 10
	_defaultQueueLen   = 1000
)

type Throttler struct {
	queue   chan struct{}
	tps     uint64
	workers uint64
}

type Option func(*Throttler)

func SetWorkerNum(n uint64) Option {
	return func(t *Throttler) {
		t.workers = n
	}
}

func SetQueueLen(n uint64) Option {
	return func(t *Throttler) {
		t.queue = make(chan struct{}, n)
	}
}

func New(tps uint64, opts ...Option) *Throttler {
	t := &Throttler{
		queue:   make(chan struct{}, _defaultQueueLen),
		tps:     tps,
		workers: _defaultNumWorkers,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *Throttler) Start(ctx context.Context) {
	go func() {
		var workers sync.WaitGroup
		ticks := make(chan uint64)
		for i := uint64(0); i < t.workers; i++ {
			workers.Add(1)
			go t.pick(&workers, ticks)
		}

		interval := uint64(time.Second.Nanoseconds() / int64(t.tps))
		began, pause := time.Now(), uint64(0)

		for {
			now, next := time.Now(), began.Add(time.Duration(pause*interval))
			began, pause = next, uint64(1)
			time.Sleep(next.Sub(now))
			select {
			case <-ctx.Done():
				return
			case ticks <- 1:
			default:
				workers.Add(1)
				go t.pick(&workers, ticks)
			}
		}
	}()
}

// Allow returns true if action is not throttled, returns false if is throttled.
func (t *Throttler) Allow() bool {
	select {
	case t.queue <- struct{}{}:
		return true
	default:
		return false
	}
}

func (t *Throttler) pick(workers *sync.WaitGroup, ticks <-chan uint64) {
	defer workers.Done()
	for range ticks {
		select {
		case <-t.queue:
		default:
		}
	}
}
