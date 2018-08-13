// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/trie"
)

type (
	// Contract is a special type of account with code and storage trie.
	Contract interface {
		GetState(key hash.Hash32B) hash.Hash32B
		SetState(key, value hash.Hash32B)
	}

	contract struct {
		State
		addrHash hash.PKHash  // 20-byte contract address hash
		root     hash.Hash32B // root of storage trie
		trie     trie.Trie    // storage trie of the contract
	}
)
