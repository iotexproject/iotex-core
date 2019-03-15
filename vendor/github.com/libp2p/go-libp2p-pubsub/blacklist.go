package pubsub

import (
	lru "github.com/hashicorp/golang-lru"
	peer "github.com/libp2p/go-libp2p-peer"
)

// Blacklist is an interface for peer blacklisting.
type Blacklist interface {
	Add(peer.ID)
	Contains(peer.ID) bool
}

// MapBlacklist is a blacklist implementation using a perfect map
type MapBlacklist map[peer.ID]struct{}

// NewMapBlacklist creates a new MapBlacklist
func NewMapBlacklist() Blacklist {
	return MapBlacklist(make(map[peer.ID]struct{}))
}

func (b MapBlacklist) Add(p peer.ID) {
	b[p] = struct{}{}
}

func (b MapBlacklist) Contains(p peer.ID) bool {
	_, ok := b[p]
	return ok
}

// LRUBlacklist is a blacklist implementation using an LRU cache
type LRUBlacklist struct {
	lru *lru.Cache
}

// NewLRUBlacklist creates a new LRUBlacklist with capacity cap
func NewLRUBlacklist(cap int) (Blacklist, error) {
	c, err := lru.New(cap)
	if err != nil {
		return nil, err
	}

	b := &LRUBlacklist{lru: c}
	return b, nil
}

func (b LRUBlacklist) Add(p peer.ID) {
	b.lru.Add(p, nil)
}

func (b LRUBlacklist) Contains(p peer.ID) bool {
	return b.lru.Contains(p)
}
