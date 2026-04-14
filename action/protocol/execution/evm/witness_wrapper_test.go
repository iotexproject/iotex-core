// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/db/trie"
)

func TestSharedRecordingKVStore(t *testing.T) {
	r := require.New(t)

	recorder := newSharedRecordingKVStore()

	addr1 := common.HexToAddress("0x1111111111111111111111111111111111111111")
	addr2 := common.HexToAddress("0x2222222222222222222222222222222222222222")

	// Record some nodes for addr1
	recorder.record([]byte("key1"), []byte("val1"), addr1)
	recorder.record([]byte("key2"), []byte("val2"), addr1)

	// Record some nodes for addr2 (with overlap on key1)
	recorder.record([]byte("key1"), []byte("val1"), addr2)
	recorder.record([]byte("key3"), []byte("val3"), addr2)

	// Check AllContracts
	contracts := recorder.AllContracts()
	r.Len(contracts, 2)

	// Check proof nodes for addr1
	nodes1 := recorder.ProofNodesForContract(addr1)
	r.Len(nodes1, 2) // key1, key2

	// Check proof nodes for addr2
	nodes2 := recorder.ProofNodesForContract(addr2)
	r.Len(nodes2, 2) // key1, key3

	// Verify deduplication: recording same key again doesn't duplicate
	recorder.record([]byte("key1"), []byte("val1_updated"), addr1)
	nodes1Again := recorder.ProofNodesForContract(addr1)
	r.Len(nodes1Again, 2) // still 2, no duplicate

	// Verify the stored value is the first one recorded
	for _, n := range nodes1Again {
		if string(n) == "val1" {
			// Found the first recorded value
			return
		}
	}
}

func TestContractRecordingKVStore(t *testing.T) {
	r := require.New(t)

	recorder := newSharedRecordingKVStore()
	addr := hash.BytesToHash160(common.HexToAddress("0x1111111111111111111111111111111111111111").Bytes())

	// Create a mock inner KVStore
	inner := newMockKVStore(map[string][]byte{
		"root":  []byte("rootnode"),
		"node1": []byte("data1"),
		"node2": []byte("data2"),
	})

	// Wrap it
	wrapFn := recorder.WrapFor(addr)
	wrapped := wrapFn(inner)

	// Get should record
	val, err := wrapped.Get([]byte("root"))
	r.NoError(err)
	r.Equal([]byte("rootnode"), val)

	val, err = wrapped.Get([]byte("node1"))
	r.NoError(err)
	r.Equal([]byte("data1"), val)

	// Put and Delete should pass through without recording
	r.NoError(wrapped.Put([]byte("new"), []byte("newval")))
	r.NoError(wrapped.Delete([]byte("node2")))

	// Check recorded nodes
	evmAddr := common.BytesToAddress(addr[:])
	nodes := recorder.ProofNodesForContract(evmAddr)
	r.Len(nodes, 2) // root, node1 (not node2 since it wasn't Get'd, not new since it was Put'd)
}

func TestWitnessStateDB_FirstTouchSemantics(t *testing.T) {
	r := require.New(t)

	addr := common.HexToAddress("0xdeadbeef00000000000000000000000000000001")
	key := common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001")
	val := common.HexToHash("0x00000000000000000000000000000000000000000000000000000000000000ff")
	rootHash := hash.Hash256{1, 2, 3}

	mock := &mockStateDB{
		states: map[common.Address]map[common.Hash]common.Hash{
			addr: {key: val},
		},
		committedStates: map[common.Address]map[common.Hash]common.Hash{
			addr: {key: val},
		},
	}

	getRootFunc := func(a common.Address) hash.Hash256 {
		if a == addr {
			return rootHash
		}
		return hash.ZeroHash256
	}

	wdb := newWitnessStateDB(mock, getRootFunc)

	// First GetState should record the prestate value
	result := wdb.GetState(addr, key)
	r.Equal(val, result)

	// Change value in underlying mock
	mock.states[addr][key] = common.HexToHash("0x00000000000000000000000000000000000000000000000000000000000001ff")

	// Second GetState should still return the new value but NOT change the recorded prestate
	result2 := wdb.GetState(addr, key)
	r.NotEqual(val, result2) // different from original

	// Entries should contain the first-touch value
	entries := wdb.WitnessEntries()
	r.Len(entries, 1)
	r.Len(entries[addr], 1)
	r.Equal(hash.BytesToHash256(key[:]), entries[addr][0].Key)
	r.Equal(val[:], entries[addr][0].Value)

	// Prestate roots should be captured
	roots := wdb.PrestateRoots()
	r.Equal(rootHash, roots[addr])
}

func TestWitnessStateDB_ZeroValueRecordedAsNil(t *testing.T) {
	r := require.New(t)

	addr := common.HexToAddress("0xdeadbeef00000000000000000000000000000002")
	key := common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001")

	mock := &mockStateDB{
		states: map[common.Address]map[common.Hash]common.Hash{
			addr: {key: common.Hash{}}, // zero value = absent
		},
		committedStates: map[common.Address]map[common.Hash]common.Hash{
			addr: {key: common.Hash{}},
		},
	}

	wdb := newWitnessStateDB(mock, func(common.Address) hash.Hash256 { return hash.ZeroHash256 })

	wdb.GetState(addr, key)

	entries := wdb.WitnessEntries()
	r.Len(entries[addr], 1)
	r.Nil(entries[addr][0].Value) // nil means absent
}

func TestWitnessStateDB_SetStateRecordsCommittedValue(t *testing.T) {
	r := require.New(t)

	addr := common.HexToAddress("0xdeadbeef00000000000000000000000000000003")
	key := common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001")
	committedVal := common.HexToHash("0x00000000000000000000000000000000000000000000000000000000000000aa")
	newVal := common.HexToHash("0x00000000000000000000000000000000000000000000000000000000000000bb")

	mock := &mockStateDB{
		states:          map[common.Address]map[common.Hash]common.Hash{addr: {key: committedVal}},
		committedStates: map[common.Address]map[common.Hash]common.Hash{addr: {key: committedVal}},
		setStates:       map[common.Address]map[common.Hash]common.Hash{},
	}

	wdb := newWitnessStateDB(mock, func(common.Address) hash.Hash256 { return hash.ZeroHash256 })

	// SetState without prior GetState should defensively record committed value
	wdb.SetState(addr, key, newVal)

	entries := wdb.WitnessEntries()
	r.Len(entries[addr], 1)
	r.Equal(committedVal[:], entries[addr][0].Value)
}

func TestAssembleWitnesses(t *testing.T) {
	r := require.New(t)

	addr := common.HexToAddress("0xdeadbeef00000000000000000000000000000004")
	key := common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001")
	val := common.HexToHash("0x00000000000000000000000000000000000000000000000000000000000000cc")
	rootHash := hash.Hash256{4, 5, 6}

	mock := &mockStateDB{
		states:          map[common.Address]map[common.Hash]common.Hash{addr: {key: val}},
		committedStates: map[common.Address]map[common.Hash]common.Hash{addr: {key: val}},
	}

	wdb := newWitnessStateDB(mock, func(a common.Address) hash.Hash256 {
		if a == addr {
			return rootHash
		}
		return hash.ZeroHash256
	})

	recorder := newSharedRecordingKVStore()
	recorder.record([]byte("proof1"), []byte("proofdata1"), addr)

	// Trigger entry capture
	wdb.GetState(addr, key)

	// Assemble
	witnesses := assembleWitnesses(wdb, recorder)
	r.Len(witnesses, 1)

	w := witnesses[addr]
	r.Equal(rootHash, w.StorageRoot)
	r.Len(w.Entries, 1)
	r.Len(w.ProofNodes, 1)
	r.Equal([]byte("proofdata1"), w.ProofNodes[0])
}

// mockKVStore is a simple in-memory trie.KVStore for testing.
type mockKVStore struct {
	data map[string][]byte
}

func newMockKVStore(data map[string][]byte) *mockKVStore {
	return &mockKVStore{data: data}
}

func (m *mockKVStore) Start(ctx context.Context) error { return nil }
func (m *mockKVStore) Stop(ctx context.Context) error  { return nil }
func (m *mockKVStore) Get(key []byte) ([]byte, error) {
	v, ok := m.data[string(key)]
	if !ok {
		return nil, trie.ErrNotExist
	}
	return v, nil
}
func (m *mockKVStore) Put(key []byte, value []byte) error {
	m.data[string(key)] = value
	return nil
}
func (m *mockKVStore) Delete(key []byte) error {
	delete(m.data, string(key))
	return nil
}

// mockStateDB is a minimal mock implementing the stateDB interface for witness tests.
type mockStateDB struct {
	stateDB // embed for interface satisfaction (will panic on unimplemented methods)
	states          map[common.Address]map[common.Hash]common.Hash
	committedStates map[common.Address]map[common.Hash]common.Hash
	setStates       map[common.Address]map[common.Hash]common.Hash
}

func (m *mockStateDB) GetState(addr common.Address, key common.Hash) common.Hash {
	if s, ok := m.states[addr]; ok {
		return s[key]
	}
	return common.Hash{}
}

func (m *mockStateDB) GetCommittedState(addr common.Address, key common.Hash) common.Hash {
	if s, ok := m.committedStates[addr]; ok {
		return s[key]
	}
	return common.Hash{}
}

func (m *mockStateDB) SetState(addr common.Address, key common.Hash, value common.Hash) {
	if m.setStates == nil {
		m.setStates = make(map[common.Address]map[common.Hash]common.Hash)
	}
	if _, ok := m.setStates[addr]; !ok {
		m.setStates[addr] = make(map[common.Hash]common.Hash)
	}
	m.setStates[addr][key] = value
	// Also update current state
	if _, ok := m.states[addr]; !ok {
		m.states[addr] = make(map[common.Hash]common.Hash)
	}
	m.states[addr][key] = value
}
