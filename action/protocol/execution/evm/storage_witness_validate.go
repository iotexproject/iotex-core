package evm

import (
	"bytes"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/db/trie/triepb"
)

// VerifyContractStorageWitness validates every witness entry against the
// contract-specific storage trie hashing rule and the supplied proof nodes.
func VerifyContractStorageWitness(addr common.Address, witness *ContractStorageWitness) error {
	if witness == nil {
		return errors.New("contract storage witness is nil")
	}
	if bytes.Equal(witness.StorageRoot[:], hash.ZeroHash256[:]) {
		// Empty trie: every entry must be an absence proof.
		for _, entry := range witness.Entries {
			if entry.Value != nil {
				return errors.Errorf("non-nil value for key %x in empty-root witness", entry.Key[:])
			}
		}
		return nil
	}
	if len(witness.ProofNodes) == 0 {
		return errors.Wrap(trie.ErrInvalidTrie, "proof is empty")
	}
	// Build the hash→node map once and reuse it for all entries.
	nodes := make(map[string][]byte, len(witness.ProofNodes))
	for _, node := range witness.ProofNodes {
		h := hash.Hash256b(append(addr.Bytes(), node...))
		nodes[string(h[:])] = node
	}
	for _, entry := range witness.Entries {
		value, err := verifyContractStorageProof(witness.StorageRoot[:], entry.Key[:], nodes)
		if entry.Value == nil {
			if errors.Cause(err) != trie.ErrNotExist {
				return errors.Wrapf(err, "failed to verify absence proof for storage key %x", entry.Key[:])
			}
			continue
		}
		if err != nil {
			return errors.Wrapf(err, "failed to verify proof for storage key %x", entry.Key[:])
		}
		if !bytes.Equal(value, entry.Value) {
			return errors.Errorf("storage witness value mismatch for key %x", entry.Key[:])
		}
	}
	return nil
}

// verifyContractStorageProof walks the pre-built nodes map from rootHash to the
// leaf matching key and returns its value, or trie.ErrNotExist for absence proofs.
func verifyContractStorageProof(rootHash []byte, key []byte, nodes map[string][]byte) ([]byte, error) {
	expectedHash := append([]byte(nil), rootHash...)
	offset := 0
	for {
		ser, ok := nodes[string(expectedHash)]
		if !ok {
			return nil, errors.Wrapf(trie.ErrInvalidTrie, "missing proof node for hash %x", expectedHash)
		}

		pb := triepb.NodePb{}
		if err := proto.Unmarshal(ser, &pb); err != nil {
			return nil, err
		}
		switch n := pb.Node.(type) {
		case *triepb.NodePb_Branch:
			if offset >= len(key) {
				return nil, errors.Wrap(trie.ErrInvalidTrie, "branch node exceeds key length")
			}
			nextHash, ok := branchChildHashForWitness(n.Branch, key[offset])
			if !ok {
				return nil, trie.ErrNotExist
			}
			expectedHash = nextHash
			offset++
		case *triepb.NodePb_Extend:
			path := n.Extend.GetPath()
			if len(path) > len(key)-offset {
				return nil, errors.Wrap(trie.ErrInvalidTrie, "extension path exceeds remaining key length")
			}
			if commonPrefixLengthForWitness(path, key[offset:]) != len(path) {
				return nil, trie.ErrNotExist
			}
			expectedHash = n.Extend.GetValue()
			offset += len(path)
		case *triepb.NodePb_Leaf:
			if len(n.Leaf.GetPath()) != len(key) {
				return nil, errors.Wrap(trie.ErrInvalidTrie, "leaf node has invalid key length")
			}
			if !bytes.Equal(n.Leaf.Path[offset:], key[offset:]) {
				return nil, trie.ErrNotExist
			}
			return n.Leaf.Value, nil
		default:
			return nil, errors.Wrap(trie.ErrInvalidTrie, "proof node has unknown type")
		}
	}
}

func branchChildHashForWitness(pb *triepb.BranchPb, index byte) ([]byte, bool) {
	for _, child := range pb.GetBranches() {
		if byte(child.GetIndex()) == index {
			return child.GetPath(), true
		}
	}
	return nil, false
}

func commonPrefixLengthForWitness(key1, key2 []byte) int {
	match := 0
	for match < len(key1) && key1[match] == key2[match] {
		match++
	}
	return match
}
