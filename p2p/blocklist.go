package p2p

import (
	"time"

	"github.com/iotexproject/go-pkgs/cache"
)

const (
	blockListLen   = 1000
	blockListTTL   = 15 * time.Minute
	blockThreshold = 3
)

// BlockList is the struct for blocklist
type BlockList struct {
	counter *cache.ThreadSafeLruCache
	timeout *cache.ThreadSafeLruCache
}

// NewBlockList creates an instance of blocklist
func NewBlockList(size int) *BlockList {
	return &BlockList{
		counter: cache.NewThreadSafeLruCache(size),
		timeout: cache.NewThreadSafeLruCache(size),
	}
}

// Blocked returns true if the name is blocked
func (bl *BlockList) Blocked(name string, t time.Time) bool {
	v, ok := bl.timeout.Get(name)
	if !ok {
		return false
	}

	if v.(time.Time).After(t) {
		return true
	}

	// timeout passed, remove name off the blocklist
	bl.Remove(name)
	return false
}

// Add tries to add the name to blocklist
func (bl *BlockList) Add(name string, t time.Time) {
	v, ok := bl.counter.Get(name)
	if !ok {
		bl.counter.Add(name, 1)
		return
	}

	// once reaching 3 faults, add to blocklist
	counter := v.(int) + 1
	bl.counter.Add(name, counter)
	if counter >= blockThreshold {
		bl.timeout.Add(name, t.Add(blockListTTL))
	}
}

// Remove takes name off the blocklist
func (bl *BlockList) Remove(name string) {
	bl.counter.Remove(name)
	bl.timeout.Remove(name)
}
