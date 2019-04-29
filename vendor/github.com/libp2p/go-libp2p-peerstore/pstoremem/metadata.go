package pstoremem

import (
	"sync"

	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
)

type metakey struct {
	id  peer.ID
	key string
}

type memoryPeerMetadata struct {
	// store other data, like versions
	//ds ds.ThreadSafeDatastore
	ds     map[metakey]interface{}
	dslock sync.RWMutex
}

var _ pstore.PeerMetadata = (*memoryPeerMetadata)(nil)

func NewPeerMetadata() pstore.PeerMetadata {
	return &memoryPeerMetadata{
		ds: make(map[metakey]interface{}),
	}
}

func (ps *memoryPeerMetadata) Put(p peer.ID, key string, val interface{}) error {
	ps.dslock.Lock()
	defer ps.dslock.Unlock()
	ps.ds[metakey{p, key}] = val
	return nil
}

func (ps *memoryPeerMetadata) Get(p peer.ID, key string) (interface{}, error) {
	ps.dslock.RLock()
	defer ps.dslock.RUnlock()
	i, ok := ps.ds[metakey{p, key}]
	if !ok {
		return nil, pstore.ErrNotFound
	}
	return i, nil
}
