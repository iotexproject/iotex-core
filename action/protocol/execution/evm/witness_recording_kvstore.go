// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/db/trie"
)

// sharedRecordingKVStore records all trie node reads (Get calls) across multiple
// contracts during a single transaction. Each recorded node is tagged with its
// originating contract address to allow per-contract proof node extraction.
//
// This is designed to be shared across all contracts in a transaction via a
// per-contract wrapper returned by WrapFor(). It sits outside the snapshot/revert
// mechanism and is append-only, so no data is lost on reverts.
type sharedRecordingKVStore struct {
	// recorded maps trie node hash → serialized node data.
	recorded map[string][]byte
	// nodeOwner maps trie node hash → set of contract addresses that accessed it.
	nodeOwner map[string]map[common.Address]struct{}
}

// newSharedRecordingKVStore creates a new shared proof node recorder.
func newSharedRecordingKVStore() *sharedRecordingKVStore {
	return &sharedRecordingKVStore{
		recorded:  make(map[string][]byte),
		nodeOwner: make(map[string]map[common.Address]struct{}),
	}
}

// record captures a trie node read, tagged with the contract address.
func (s *sharedRecordingKVStore) record(key []byte, value []byte, addr common.Address) {
	k := string(key)
	if _, ok := s.recorded[k]; !ok {
		s.recorded[k] = append([]byte(nil), value...)
	}
	owners, ok := s.nodeOwner[k]
	if !ok {
		owners = make(map[common.Address]struct{})
		s.nodeOwner[k] = owners
	}
	owners[addr] = struct{}{}
}

// ProofNodesForContract returns the proof nodes (serialized trie node data)
// that were accessed by the given contract address.
func (s *sharedRecordingKVStore) ProofNodesForContract(addr common.Address) [][]byte {
	var nodes [][]byte
	for k, owners := range s.nodeOwner {
		if _, ok := owners[addr]; ok {
			nodes = append(nodes, s.recorded[k])
		}
	}
	return nodes
}

// AllContracts returns all contract addresses that had trie node reads recorded.
func (s *sharedRecordingKVStore) AllContracts() []common.Address {
	seen := make(map[common.Address]struct{})
	for _, owners := range s.nodeOwner {
		for addr := range owners {
			seen[addr] = struct{}{}
		}
	}
	addrs := make([]common.Address, 0, len(seen))
	for addr := range seen {
		addrs = append(addrs, addr)
	}
	return addrs
}

// WrapFor returns a trie.KVStore wrapper for the given contract address.
// All Get calls through the wrapper are forwarded to the inner KVStore and
// recorded in the shared store tagged with the contract address.
func (s *sharedRecordingKVStore) WrapFor(addr hash.Hash160) func(trie.KVStore) trie.KVStore {
	evmAddr := common.BytesToAddress(addr[:])
	return func(inner trie.KVStore) trie.KVStore {
		return &contractRecordingKVStore{
			inner:  inner,
			shared: s,
			addr:   evmAddr,
		}
	}
}

// contractRecordingKVStore is a per-contract trie.KVStore wrapper that
// delegates all operations to inner and records Get calls to the shared store.
type contractRecordingKVStore struct {
	inner  trie.KVStore
	shared *sharedRecordingKVStore
	addr   common.Address
}

func (r *contractRecordingKVStore) Start(ctx context.Context) error { return r.inner.Start(ctx) }
func (r *contractRecordingKVStore) Stop(ctx context.Context) error  { return r.inner.Stop(ctx) }

func (r *contractRecordingKVStore) Get(key []byte) ([]byte, error) {
	v, err := r.inner.Get(key)
	if err == nil {
		r.shared.record(key, v, r.addr)
	}
	return v, err
}

func (r *contractRecordingKVStore) Put(key []byte, value []byte) error {
	return r.inner.Put(key, value)
}

func (r *contractRecordingKVStore) Delete(key []byte) error {
	return r.inner.Delete(key)
}

// prime forces a Get on the given key so that the trie root node is recorded.
// After Clone, the root is already in memory and won't be loaded through Get
// during normal operations. This ensures the root is captured.
func (r *contractRecordingKVStore) prime(key []byte) {
	r.Get(key)
}
