package p2p

import (
	"sync"

	"go.uber.org/zap"
)

// logger is the logger instance
var (
	_loggerMu  sync.RWMutex
	_logger, _ = zap.NewDevelopment()
)

// Logger returns the logger
func Logger() *zap.Logger {
	_loggerMu.RLock()
	l := _logger
	_loggerMu.RUnlock()
	return l
}

// SetLogger sets the logger
func SetLogger(l *zap.Logger) {
	_loggerMu.Lock()
	_logger = l
	_loggerMu.Unlock()
}
