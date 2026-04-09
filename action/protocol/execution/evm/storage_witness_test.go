package evm

import (
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"
)

func TestContractStorageWitness_MergeFrom(t *testing.T) {
	require := require.New(t)

	k1 := hash.Hash256b([]byte("key1"))
	k2 := hash.Hash256b([]byte("key2"))
	k3 := hash.Hash256b([]byte("key3"))
	v1 := []byte("value1")
	v2 := []byte("value2")

	nodeA := []byte("proof-node-a")
	nodeB := []byte("proof-node-b")
	nodeC := []byte("proof-node-c")

	w1 := &ContractStorageWitness{
		StorageRoot: hash.Hash256b([]byte("root")),
		Entries: []ContractStorageWitnessEntry{
			{Key: k1, Value: v1},
			{Key: k2, Value: v2},
		},
		ProofNodes: [][]byte{nodeA, nodeB},
	}

	w2 := &ContractStorageWitness{
		StorageRoot: hash.Hash256b([]byte("root")),
		Entries: []ContractStorageWitnessEntry{
			{Key: k2, Value: v2}, // duplicate
			{Key: k3, Value: nil},
		},
		ProofNodes: [][]byte{nodeB, nodeC}, // nodeB is duplicate
	}

	w1.MergeFrom(w2)

	// Entries: k1, k2 (original), k3 (new from w2). k2 not duplicated.
	require.Len(w1.Entries, 3)
	entryKeys := make(map[hash.Hash256]bool)
	for _, e := range w1.Entries {
		entryKeys[e.Key] = true
	}
	require.True(entryKeys[k1])
	require.True(entryKeys[k2])
	require.True(entryKeys[k3])

	// ProofNodes: nodeA, nodeB (original), nodeC (new from w2). nodeB not duplicated.
	require.Len(w1.ProofNodes, 3)
	nodeSet := make(map[string]bool)
	for _, n := range w1.ProofNodes {
		nodeSet[string(n)] = true
	}
	require.True(nodeSet[string(nodeA)])
	require.True(nodeSet[string(nodeB)])
	require.True(nodeSet[string(nodeC)])
}

func TestContractStorageWitness_MergeFromEmpty(t *testing.T) {
	require := require.New(t)

	k1 := hash.Hash256b([]byte("key1"))

	w1 := &ContractStorageWitness{
		StorageRoot: hash.Hash256b([]byte("root")),
		Entries: []ContractStorageWitnessEntry{
			{Key: k1, Value: []byte("v1")},
		},
		ProofNodes: [][]byte{[]byte("node1")},
	}

	w2 := &ContractStorageWitness{
		StorageRoot: hash.Hash256b([]byte("root")),
	}

	w1.MergeFrom(w2)
	require.Len(w1.Entries, 1)
	require.Len(w1.ProofNodes, 1)
}
