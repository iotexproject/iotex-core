package evm

import (
	"context"

	"github.com/iotexproject/iotex-core/v2/db/trie"
)

// mirroredTrieKVStore keeps proof nodes in memory for stateless reads while
// mirroring trie mutations into the state manager-backed KV store so the
// working-set digest matches full execution.
type mirroredTrieKVStore struct {
	primary trie.KVStore
	mirror  trie.KVStore
}

func newMirroredTrieKVStore(primary trie.KVStore, mirror trie.KVStore) trie.KVStore {
	return &mirroredTrieKVStore{
		primary: primary,
		mirror:  mirror,
	}
}

func (s *mirroredTrieKVStore) Start(ctx context.Context) error {
	if err := s.primary.Start(ctx); err != nil {
		return err
	}
	return s.mirror.Start(ctx)
}

func (s *mirroredTrieKVStore) Stop(ctx context.Context) error {
	if err := s.primary.Stop(ctx); err != nil {
		return err
	}
	return s.mirror.Stop(ctx)
}

func (s *mirroredTrieKVStore) Put(key []byte, value []byte) error {
	if err := s.primary.Put(key, value); err != nil {
		return err
	}
	return s.mirror.Put(key, value)
}

func (s *mirroredTrieKVStore) Delete(key []byte) error {
	if err := s.primary.Delete(key); err != nil {
		return err
	}
	return s.mirror.Delete(key)
}

func (s *mirroredTrieKVStore) Get(key []byte) ([]byte, error) {
	return s.primary.Get(key)
}
