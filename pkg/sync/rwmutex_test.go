package sync

import (
	"testing"

	"go.uber.org/zap"
)

func TestRWMutex(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		rs := NewRWMutex()
		rs.Lock()
		rs.Unlock()
		rs.RLock()
		rs.RUnlock()
	})
	t.Run("with logger", func(t *testing.T) {
		rs := NewRWMutex(WithLogger(func() *zap.Logger {
			return zap.NewExample()
		}))
		rs.Lock()
		rs.Unlock()
		rs.RLock()
		rs.RUnlock()
	})
}
