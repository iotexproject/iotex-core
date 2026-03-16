// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"context"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/db/trie/mptrie"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

const (
	// CodeKVNameSpace is the bucket name for code
	CodeKVNameSpace = state.CodeKVNameSpace
	// ContractKVNameSpace is the bucket name for contract data storage
	ContractKVNameSpace = state.ContractKVNameSpace
	// PreimageKVNameSpace is the bucket name for preimage data storage
	PreimageKVNameSpace = state.PreimageKVNameSpace
)

type (
	// Contract is a special type of account with code and storage trie.
	Contract interface {
		GetCommittedState(hash.Hash256) ([]byte, error)
		GetState(hash.Hash256) ([]byte, error)
		SetState(hash.Hash256, []byte) error
		BuildStorageWitness(ContractStorageAccess) (*ContractStorageWitness, error)
		GetCode() ([]byte, error)
		SetCode(hash.Hash256, []byte)
		SelfState() *state.Account
		Commit() error
		LoadRoot() error
		Iterator() (trie.Iterator, error)
		Snapshot() Contract
	}

	contract struct {
		*state.Account
		addr       hash.Hash160
		async      bool
		dirtyCode  bool                       // contract's code has been set
		dirtyState bool                       // contract's account state has changed
		code       protocol.SerializableBytes // contract byte-code
		root       hash.Hash256
		committed  map[hash.Hash256][]byte
		missing    map[hash.Hash256]struct{}
		sm         protocol.StateManager
		trie       trie.Trie      // storage trie of the contract
		prestate   trie.ProofTrie // snapshot captured before the first storage mutation
	}
)

func (c *contract) Iterator() (trie.Iterator, error) {
	return mptrie.NewLeafIterator(c.trie)
}

// GetCommittedState get the committed value of a key
func (c *contract) GetCommittedState(key hash.Hash256) ([]byte, error) {
	if v, ok := c.committed[key]; ok {
		return v, nil
	}
	return c.GetState(key)
}

// GetState get the value from contract storage
func (c *contract) GetState(key hash.Hash256) ([]byte, error) {
	v, err := c.trie.Get(key[:])
	if err != nil {
		if errors.Cause(err) == trie.ErrNotExist {
			c.missing[key] = struct{}{}
		}
		return nil, err
	}
	delete(c.missing, key)
	if _, ok := c.committed[key]; !ok {
		c.committed[key] = v
	}
	return v, nil
}

// SetState set the value into contract storage
func (c *contract) SetState(key hash.Hash256, value []byte) error {
	if _, ok := c.committed[key]; !ok {
		_, _ = c.GetState(key)
	}
	if err := c.capturePrestateTrie(); err != nil {
		return err
	}
	c.dirtyState = true
	if err := c.trie.Upsert(key[:], value); err != nil {
		return err
	}
	if !c.async {
		rh, err := c.trie.RootHash()
		if err != nil {
			return err
		}
		// TODO (zhi): confirm whether we should update the root on err
		c.Account.Root = hash.BytesToHash256(rh)
	}

	return nil
}

func (c *contract) BuildStorageWitness(access ContractStorageAccess) (*ContractStorageWitness, error) {
	pt, err := c.proofTrieForWitness()
	if err != nil {
		return nil, err
	}
	keys := touchedStorageKeys(access)
	entries := make([]ContractStorageWitnessEntry, 0, len(keys))
	proofNodes := make([][]byte, 0)
	seenProofNodes := make(map[string]struct{})
	for _, key := range keys {
		slotKey := hash.BytesToHash256(key[:])
		prestateValue, ok := c.committed[slotKey]
		if !ok {
			if _, missing := c.missing[slotKey]; !missing {
				return nil, errors.Errorf("missing committed pre-state for storage key %x", key[:])
			}
		}
		entry := ContractStorageWitnessEntry{
			Key:   slotKey,
			Value: cloneBytes(prestateValue),
		}
		entries = append(entries, entry)

		proof, err := pt.GetProof(key[:])
		if err != nil && errors.Cause(err) != trie.ErrNotExist {
			return nil, err
		}
		for _, node := range proof {
			nodeKey := string(node)
			if _, ok := seenProofNodes[nodeKey]; ok {
				continue
			}
			seenProofNodes[nodeKey] = struct{}{}
			proofNodes = append(proofNodes, cloneBytes(node))
		}
	}

	return &ContractStorageWitness{
		StorageRoot: c.root,
		Entries:     entries,
		ProofNodes:  proofNodes,
	}, nil
}

func (c *contract) proofTrieForWitness() (trie.ProofTrie, error) {
	if c.prestate != nil {
		return c.prestate, nil
	}
	if c.dirtyState {
		return nil, errors.New("missing pre-state trie snapshot for witness assembly")
	}
	pt, ok := c.trie.(trie.ProofTrie)
	if !ok {
		return nil, errors.New("contract storage trie does not support proofs")
	}
	return pt, nil
}

func (c *contract) capturePrestateTrie() error {
	if c.prestate != nil {
		return nil
	}
	snapshot, err := newStorageTrie(c.addr, trie.NewMemKVStore(), false)
	if err != nil {
		return err
	}
	iter, err := c.Iterator()
	if err != nil {
		return err
	}
	for {
		key, value, err := iter.Next()
		if err != nil {
			if errors.Cause(err) == trie.ErrEndOfIterator {
				break
			}
			return err
		}
		if err := snapshot.Upsert(key, value); err != nil {
			return err
		}
	}
	pt, ok := snapshot.(trie.ProofTrie)
	if !ok {
		return errors.New("contract storage trie does not support proofs")
	}
	c.prestate = pt
	return nil
}

// GetCode gets the contract's byte-code
func (c *contract) GetCode() ([]byte, error) {
	if c.code != nil {
		return c.code[:], nil
	}
	_, err := c.sm.State(&c.code, protocol.NamespaceOption(CodeKVNameSpace), protocol.KeyOption(c.Account.CodeHash))
	if err != nil {
		return nil, err
	}
	return c.code[:], nil

}

// SetCode sets the contract's byte-code
func (c *contract) SetCode(hash hash.Hash256, code []byte) {
	c.Account.CodeHash = hash[:]
	c.code = code
	c.dirtyCode = true
}

// account returns this contract's account
func (c *contract) SelfState() *state.Account {
	return c.Account
}

// Commit writes the changes into underlying trie
func (c *contract) Commit() error {
	if c.dirtyState {
		rh, err := c.trie.RootHash()
		if err != nil {
			return err
		}
		// record the new root hash, global account trie will Commit all pending writes to DB
		c.Account.Root = hash.BytesToHash256(rh)
		c.root = c.Account.Root
		c.dirtyState = false
		// purge the committed value cache
		c.committed = nil
		c.committed = make(map[hash.Hash256][]byte)
		c.missing = nil
		c.missing = make(map[hash.Hash256]struct{})
		c.prestate = nil
	}
	if c.dirtyCode {
		if _, err := c.sm.PutState(c.code, protocol.NamespaceOption(CodeKVNameSpace), protocol.KeyOption(c.Account.CodeHash)); err != nil {
			return errors.Wrapf(err, "Failed to store code for new contract, codeHash %x", c.Account.CodeHash[:])
		}
		c.dirtyCode = false
	}
	return nil
}

// LoadRoot loads storage trie's root
func (c *contract) LoadRoot() error {
	return c.trie.SetRootHash(c.Account.Root[:])
}

// Snapshot takes a snapshot of the contract object
func (c *contract) Snapshot() Contract {
	if c.async {
		rh, err := c.trie.RootHash()
		if err != nil {
			log.L().Fatal("failed to calculate root hash")
		}
		c.Account.Root = hash.BytesToHash256(rh)
	}
	return &contract{
		Account:    c.Account.Clone(),
		addr:       c.addr,
		async:      c.async,
		dirtyCode:  c.dirtyCode,
		dirtyState: c.dirtyState,
		code:       c.code,
		root:       c.Account.Root,
		committed:  c.committed,
		missing:    c.missing,
		sm:         c.sm,
		prestate:   c.prestate,
		// note we simply save the trie (which is an interface/pointer)
		// later Revert() call needs to reset the saved trie root
		trie: c.trie,
	}
}

// newContract returns a Contract instance
func newContract(addr hash.Hash160, account *state.Account, sm protocol.StateManager, enableAsync bool) (Contract, error) {
	c := &contract{
		Account:   account,
		addr:      addr,
		root:      account.Root,
		committed: make(map[hash.Hash256][]byte),
		missing:   make(map[hash.Hash256]struct{}),
		sm:        sm,
		async:     enableAsync,
	}
	tr, err := newStorageTrie(addr, protocol.NewKVStoreForTrieWithStateManager(ContractKVNameSpace, sm), enableAsync)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create storage trie for new contract")
	}
	if account.Root != hash.ZeroHash256 {
		if err := tr.SetRootHash(account.Root[:]); err != nil {
			return nil, err
		}
	}
	c.trie = tr
	return c, nil
}

func newStorageTrie(addr hash.Hash160, kvStore trie.KVStore, enableAsync bool) (trie.Trie, error) {
	options := []mptrie.Option{
		mptrie.KVStoreOption(kvStore),
		mptrie.KeyLengthOption(len(hash.Hash256{})),
		mptrie.HashFuncOption(func(data []byte) []byte {
			h := hash.Hash256b(append(addr[:], data...))
			return h[:]
		}),
	}
	if enableAsync {
		options = append(options, mptrie.AsyncOption())
	}
	tr, err := mptrie.New(options...)
	if err != nil {
		return nil, err
	}
	if err := tr.Start(context.Background()); err != nil {
		return nil, err
	}
	return tr, nil
}
