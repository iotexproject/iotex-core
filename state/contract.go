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
		SelfState() *AccountState
		Commit() error
		RootHash() hash.Hash32B
	}

	contract struct {
		*AccountState
		dirtyCode  bool      // contract's code has been set
		dirtyState bool      // contract's AccountState has changed
		code       []byte    // contract byte-code
		trie       trie.Trie // storage trie of the contract
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
	c.dirtyState = true
	return c.trie.Upsert(key[:], value)
}

// GetCode gets the contract's byte-code
func (c *contract) GetCode() ([]byte, error) {
	if c.code != nil {
		return c.code, nil
	}
	return c.trie.TrieDB().Get(trie.CodeKVNameSpace, c.AccountState.CodeHash[:])
}

// SetCode sets the contract's byte-code
func (c *contract) SetCode(hash hash.Hash32B, code []byte) {
	c.AccountState.CodeHash = hash[:]
	c.code = code
	c.dirtyCode = true
}

// AccountState returns this contract's AccountState
func (c *contract) SelfState() *AccountState {
	return c.AccountState
}

// Commit writes the changes into underlying trie
func (c *contract) Commit() error {
	if c.dirtyState {
		// record the new root hash, global account trie will Commit all pending writes to DB
		c.AccountState.Root = c.trie.RootHash()
		c.dirtyState = false
	}
	if c.dirtyCode {
		// put the code into storage DB
		if err := c.trie.TrieDB().Put(trie.CodeKVNameSpace, c.AccountState.CodeHash[:], c.code); err != nil {
			return errors.Wrapf(err, "Failed to store code for new contract, codeHash %x", c.AccountState.CodeHash[:])
		}
		c.dirtyCode = false
	}
	return nil
}

// RootHash returns storage trie's root hash
func (c *contract) RootHash() hash.Hash32B {
	return c.AccountState.Root
}

// newContract returns a Contract instance
func newContract(state *AccountState, tr trie.Trie) Contract {
	c := contract{
		AccountState: state,
		trie:         tr,
	}
	c.trie.Start(context.Background())
	return &c
}
