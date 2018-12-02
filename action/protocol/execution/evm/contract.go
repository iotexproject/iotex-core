// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package evm

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state"
)

const (
	// CodeKVNameSpace is the bucket name for code
	CodeKVNameSpace = "Code"

	// ContractKVNameSpace is the bucket name for contract data storage
	ContractKVNameSpace = "Contract"
)

type (
	// Contract is a special type of account with code and storage trie.
	Contract interface {
		GetState(hash.Hash32B) ([]byte, error)
		SetState(hash.Hash32B, []byte) error
		GetCode() ([]byte, error)
		SetCode(hash.Hash32B, []byte)
		SelfState() *state.Account
		Commit() error
		RootHash() hash.Hash32B
	}

	contract struct {
		*state.Account
		dirtyCode  bool      // contract's code has been set
		dirtyState bool      // contract's account state has changed
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
	return c.trie.TrieDB().Get(CodeKVNameSpace, c.Account.CodeHash)
}

// SetCode sets the contract's byte-code
func (c *contract) SetCode(hash hash.Hash32B, code []byte) {
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
		// record the new root hash, global account trie will Commit all pending writes to DB
		c.Account.Root = c.trie.RootHash()
		c.dirtyState = false
	}
	if c.dirtyCode {
		// put the code into storage DB
		if err := c.trie.TrieDB().Put(CodeKVNameSpace, c.Account.CodeHash, c.code); err != nil {
			return errors.Wrapf(err, "Failed to store code for new contract, codeHash %x", c.Account.CodeHash[:])
		}
		c.dirtyCode = false
	}
	return nil
}

// RootHash returns storage trie's root hash
func (c *contract) RootHash() hash.Hash32B {
	return c.Account.Root
}

// newContract returns a Contract instance
func newContract(state *state.Account, tr trie.Trie) Contract {
	c := contract{
		Account: state,
		trie:    tr,
	}
	c.trie.Start(context.Background())
	return &c
}
