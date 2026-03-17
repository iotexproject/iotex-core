package evm

import (
	"bytes"
	"context"

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
	tr, err := newStorageTrie(hash.BytesToHash160(addr.Bytes()), trie.NewMemKVStore(), false)
	if err != nil {
		return err
	}
	defer func() {
		_ = tr.Stop(context.Background())
	}()

	if _, ok := tr.(trie.ProofTrie); !ok {
		return errors.New("contract storage trie does not support proofs")
	}
	for _, entry := range witness.Entries {
		value, err := verifyContractStorageProof(addr, witness.StorageRoot[:], entry.Key[:], witness.ProofNodes)
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

func verifyContractStorageProof(addr common.Address, rootHash []byte, key []byte, proof [][]byte) ([]byte, error) {
	if bytes.Equal(rootHash, hash.ZeroHash256[:]) {
		return nil, trie.ErrNotExist
	}
	if len(proof) == 0 {
		return nil, errors.Wrap(trie.ErrInvalidTrie, "proof is empty")
	}
	nodes := make(map[string][]byte, len(proof))
	for _, node := range proof {
		h := hash.Hash256b(append(addr.Bytes(), node...))
		nodes[string(h[:])] = node
	}

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
