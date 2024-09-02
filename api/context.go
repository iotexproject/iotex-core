package api

import (
	"context"
	"sync"
)

type (
	streamContextKey struct{}

	StreamContext struct {
		listenerIDs map[string]struct{}
		mutex       sync.Mutex
	}
)

func (sc *StreamContext) AddListener(id string) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	sc.listenerIDs[id] = struct{}{}
}

func (sc *StreamContext) RemoveListener(id string) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	delete(sc.listenerIDs, id)
}

func (sc *StreamContext) ListenerIDs() []string {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	ids := make([]string, 0, len(sc.listenerIDs))
	for id := range sc.listenerIDs {
		ids = append(ids, id)
	}
	return ids
}

func WithStreamContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, streamContextKey{}, &StreamContext{
		listenerIDs: make(map[string]struct{}),
	})
}

func StreamFromContext(ctx context.Context) (*StreamContext, bool) {
	sc, ok := ctx.Value(streamContextKey{}).(*StreamContext)
	return sc, ok
}
