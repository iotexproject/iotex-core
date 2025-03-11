package sync

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

type (
	// RWMutex is a wrapper around sync.RWMutex that logs lock and unlock events.
	RWMutex struct {
		rw       sync.RWMutex
		loggerFn func() *zap.Logger
		uuid     int64
	}
	// RWMutexOption is a function that configures an RWMutex.
	RWMutexOption func(*RWMutex)
)

// WithLogger sets the logger used by the RWMutex.
func WithLogger(loggerFn func() *zap.Logger) RWMutexOption {
	return func(lm *RWMutex) {
		lm.loggerFn = loggerFn
	}
}

// NewRWMutex creates a new RWMutex with the provided options.
func NewRWMutex(opts ...RWMutexOption) *RWMutex {
	rw := &RWMutex{
		uuid:     time.Now().Unix(),
		loggerFn: func() *zap.Logger { return zap.NewNop() },
	}
	for _, opt := range opts {
		opt(rw)
	}
	return rw
}

// Lock locks the mutex.
func (lm *RWMutex) Lock() {
	lm.logger().Debug("request lock")
	lm.rw.Lock()
	lm.logger().Debug("acquired lock")
}

// Unlock unlocks the mutex.
func (lm *RWMutex) Unlock() {
	lm.logger().Debug("request unlock")
	lm.rw.Unlock()
	lm.logger().Debug("released lock")
}

// RLock locks the mutex for reading.
func (lm *RWMutex) RLock() {
	lm.logger().Debug("request rlock")
	lm.rw.RLock()
	lm.logger().Debug("acquired rlock")
}

// RUnlock unlocks the mutex for reading.
func (lm *RWMutex) RUnlock() {
	lm.logger().Debug("request runlock")
	lm.rw.RUnlock()
	lm.logger().Debug("released rlock")
}

func (lm *RWMutex) logger() *zap.Logger {
	return lm.loggerFn().WithOptions(zap.AddCaller(), zap.AddCallerSkip(1)).With(zap.Int64("uuid", lm.uuid))
}
