// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"context"
)

// KVStore defines an interface for storing trie data as key-value pair
type KVStore interface {
	// Start starts the KVStore
	Start(context.Context) error
	// Stop stops the KVStore
	Stop(context.Context) error
	// Put puts key, value pair into KVStore
	Put([]byte, []byte) error
	// Delete deletes record from KVStore by key
	Delete([]byte) error
	// Get gets the value from KVStore by key
	Get([]byte) ([]byte, error)
}
