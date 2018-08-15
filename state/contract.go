// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/trie"
)

type (
	// Contract is a special type of account with code and storage trie.
	Contract interface {
		GetState(hash.Hash32B) ([]byte, error)
		SetState(hash.Hash32B, []byte) error
		GetCode() ([]byte, error)
		SetCode(hash.Hash32B, []byte)
		SelfState() *State
		Commit() error
		RootHash() hash.Hash32B
	}

	contract struct {
		*State
		code []byte    // contract byte-code
		trie trie.Trie // storage trie of the contract
	}
)

// GetState get the value from contract storage
func (c *contract) GetState(key hash.Hash32B) ([]byte, error) {
	v, err := c.trie.Get(key[:])
	if err != nil {
		return nil, err
	}
	return v, nil
}

// SetState set the value into contract storage
func (c *contract) SetState(key hash.Hash32B, value []byte) error {
	return c.trie.Upsert(key[:], value)
}

// GetCode gets the contract's byte-code
func (c *contract) GetCode() ([]byte, error) {
	if c.code != nil {
		return c.code, nil
	}
	code, err := c.trie.TrieDB().Get(trie.CodeKVNameSpace, c.State.CodeHash[:])
	if err != nil {
		return nil, err
	}
	return code, nil
}

// SetCode sets the contract's byte-code
func (c *contract) SetCode(hash hash.Hash32B, code []byte) {
	c.State.CodeHash = hash
	c.code = code
}

// State returns this contract's state
func (c *contract) SelfState() *State {
	return c.State
}

// Commit writes the changes into underlying trie
func (c *contract) Commit() error {
	if c.State.CodeHash != hash.ZeroHash32B {
		// put the code into storage DB
		if err := c.trie.TrieDB().Put(trie.CodeKVNameSpace, c.State.CodeHash[:], c.code); err != nil {
			return errors.Wrapf(err, "Failed to store code for new contract, codeHash %x", c.State.CodeHash[:])
		}
	}
	c.State.Root = c.trie.RootHash()
	return c.trie.Commit()
}

// RootHash returns storage trie's root hash
func (c *contract) RootHash() hash.Hash32B {
	return c.State.Root
}

// newContract returns a Contract instance
func newContract(state *State, code []byte, tr trie.Trie) Contract {
	c := contract{
		State: state,
		code:  code,
		trie:  tr,
	}
	c.trie.Start(context.Background())
	c.trie.EnableBatch()
	return &c
}
