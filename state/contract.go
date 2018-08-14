// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"context"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/trie"
)

type (
	// Contract is a special type of account with code and storage trie.
	Contract interface {
		GetState(key hash.Hash32B) ([]byte, error)
		SetState(key hash.Hash32B, value []byte) error
		SelfState() *State
		Commit() error
		RootHash() hash.Hash32B
	}

	contract struct {
		State
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

// State returns this contract's state
func (c *contract) SelfState() *State {
	return &c.State
}

// Commit writes the changes into underlying trie
func (c *contract) Commit() error {
	c.State.Root = c.trie.RootHash()
	return c.trie.Commit()
}

// RootHash returns storage trie's root hash
func (c *contract) RootHash() hash.Hash32B {
	return c.State.Root
}

// newContract returns a Contract instance
func newContract(tr trie.Trie, state *State, root hash.Hash32B) Contract {
	c := contract{
		State: *state,
		trie:  tr,
	}
	c.trie.Start(context.Background())
	return &c
}
