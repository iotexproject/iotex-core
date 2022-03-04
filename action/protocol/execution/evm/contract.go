// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package evm

import (
	"context"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/db/trie/mptrie"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
)

const (
	// CodeKVNameSpace is the bucket name for code
	CodeKVNameSpace = "Code"
	// ContractKVNameSpace is the bucket name for contract data storage
	ContractKVNameSpace = "Contract"
	// PreimageKVNameSpace is the bucket name for preimage data storage
	PreimageKVNameSpace = "Preimage"
)

type (
	// Contract is a special type of account with code and storage trie.
	Contract interface {
		GetCommittedState(hash.Hash256) ([]byte, error)
		GetState(hash.Hash256) ([]byte, error)
		SetState(hash.Hash256, []byte) error
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
		async      bool
		dirtyCode  bool                       // contract's code has been set
		dirtyState bool                       // contract's account state has changed
		code       protocol.SerializableBytes // contract byte-code
		root       hash.Hash256
		committed  map[hash.Hash256][]byte
		sm         protocol.StateManager
		trie       trie.Trie // storage trie of the contract
	}
)

func (c *contract) Iterator() (trie.Iterator, error) {
	return mptrie.NewLeafIterator(c.trie)
}

// GetState get the committed value of a key
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
		return nil, err
	}
	if _, ok := c.committed[key]; !ok {
		c.committed[key] = v
	}
	return v, nil
}

// SetState set the value into contract storage
func (c *contract) SetState(key hash.Hash256, value []byte) error {
	if _, ok := c.committed[key]; !ok {
		c.GetState(key)
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
		c.dirtyState = false
		// purge the committed value cache
		c.committed = nil
		c.committed = make(map[hash.Hash256][]byte)
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
		async:      c.async,
		dirtyCode:  c.dirtyCode,
		dirtyState: c.dirtyState,
		code:       c.code,
		root:       c.Account.Root,
		committed:  c.committed,
		sm:         c.sm,
		// note we simply save the trie (which is an interface/pointer)
		// later Revert() call needs to reset the saved trie root
		trie: c.trie,
	}
}

// newContract returns a Contract instance
func newContract(addr hash.Hash160, account *state.Account, sm protocol.StateManager, enableAsync bool) (Contract, error) {
	c := &contract{
		Account:   account,
		root:      account.Root,
		committed: make(map[hash.Hash256][]byte),
		sm:        sm,
		async:     enableAsync,
	}
	options := []mptrie.Option{
		mptrie.KVStoreOption(protocol.NewKVStoreForTrieWithStateManager(ContractKVNameSpace, sm)),
		mptrie.KeyLengthOption(len(hash.Hash256{})),
		mptrie.HashFuncOption(func(data []byte) []byte {
			h := hash.Hash256b(append(addr[:], data...))
			return h[:]
		}),
	}
	if account.Root != hash.ZeroHash256 {
		options = append(options, mptrie.RootHashOption(account.Root[:]))
	}

	tr, err := mptrie.New(options...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create storage trie for new contract")
	}
	if err := tr.Start(context.Background()); err != nil {
		return nil, err
	}
	c.trie = tr
	return c, nil
}
