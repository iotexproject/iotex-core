package api

import (
	"context"
	"sync"
)

type (
	streamContextKey struct{}

	streamListener struct {
		done   <-chan struct{}
		remove func()
	}

	StreamContext struct {
		listenerIDs map[string]streamListener
		mutex       sync.Mutex
	}

	apiContextKey struct{}
)

func (sc *StreamContext) AddListener(id string) {
	sc.addListener(id, nil, nil)
}

// addListener tracks the sender lifecycle as well as the subscription ID.
// A dropped sender frees its connection slot even before the next block evicts
// the responder from the chain listener.
func (sc *StreamContext) addListener(id string, done <-chan struct{}, remove func()) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	sc.listenerIDs[id] = streamListener{done: done, remove: remove}
}

func (sc *StreamContext) RemoveListener(id string) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	delete(sc.listenerIDs, id)
}

// ListenerCount returns the number of listeners held by the stream
func (sc *StreamContext) ListenerCount() int {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	sc.pruneListeners()
	return len(sc.listenerIDs)
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

// pruneListeners requires sc.mutex. Nil channels represent listeners whose
// lifecycle is managed explicitly with RemoveListener.
// Calling RemoveResponder while holding sc.mutex relies on chainListener never
// calling back into StreamContext; Exit must remain non-blocking.
func (sc *StreamContext) pruneListeners() {
	for id, listener := range sc.listenerIDs {
		select {
		case <-listener.done:
			if listener.remove != nil {
				listener.remove()
			}
			delete(sc.listenerIDs, id)
		default:
		}
	}
}

func WithStreamContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, streamContextKey{}, &StreamContext{
		listenerIDs: make(map[string]streamListener),
	})
}

func StreamFromContext(ctx context.Context) (*StreamContext, bool) {
	sc, ok := ctx.Value(streamContextKey{}).(*StreamContext)
	return sc, ok
}

func WithAPIContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, apiContextKey{}, struct{}{})
}

func GetAPIContext(ctx context.Context) (struct{}, bool) {
	c := ctx.Value(apiContextKey{})
	if c == nil {
		return struct{}{}, false
	}
	return c.(struct{}), true
}
