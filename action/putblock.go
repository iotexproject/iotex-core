// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
)

// PutBlock represents put a sub-chain block message
type PutBlock struct {
	abstractAction
	chainID            uint32
	height             uint64
	hash               hash.Hash32B
	actionRoot         hash.Hash32B
	stateRoot          hash.Hash32B
	endorsorSignatures map[keypair.PublicKey][]byte
}

// Hash returns the hash of putting a sub-chain block message
func (put *PutBlock) Hash() hash.Hash32B {
	// TODO: implement hash generation
	var hash hash.Hash32B
	return hash
}
