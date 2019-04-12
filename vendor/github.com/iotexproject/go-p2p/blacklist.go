package p2p

import (
	lru "github.com/hashicorp/golang-lru"
	peer "github.com/libp2p/go-libp2p-peer"
)

// LRUBlacklist is a blacklist implementation using an LRU cache
type LRUBlacklist struct {
	lru *lru.Cache
}

// NewLRUBlacklist creates a new LRUBlacklist with capacity cap
func NewLRUBlacklist(cap int) (*LRUBlacklist, error) {
	c, err := lru.New(cap)
	if err != nil {
		return nil, err
	}

	b := &LRUBlacklist{lru: c}
	return b, nil
}

// Add adds a peer ID
func (b *LRUBlacklist) Add(p peer.ID) {
	b.lru.Add(p, nil)
}

// Remove removes a peer ID
func (b *LRUBlacklist) Remove(p peer.ID) {
	b.lru.Remove(p)
}

// RemoveOldest removes the oldest peer ID
func (b *LRUBlacklist) RemoveOldest() {
	b.lru.RemoveOldest()
}

// Contains checks if the peer ID is in LRU
func (b *LRUBlacklist) Contains(p peer.ID) bool {
	return b.lru.Contains(p)
}
