// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package crypto

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/pkg/hash"
)

func decodeHash(in string) [32]byte {
	hash, _ := hex.DecodeString(in)
	var arr [32]byte
	copy(arr[:], hash[:32])
	return arr
}

func TestMerkleTree(t *testing.T) {
	var inputs []hash.Hash256

	// with no leave
	m := NewMerkleTree(inputs)
	assert.Nil(t, m)

	inputs = []hash.Hash256{
		decodeHash("aeedd06eb44f08abbcc72a2293aff580f13662fa59cc1b0aa4a15ee7c118e4eb"),
		decodeHash("9de6306b08158c423330f7a27243a1a5cbe39bfd764f07818437882d21241567"),
		decodeHash("7959228bfdb316949973c08d8bb7bea2a21227a7b4ed85c35d247bf3d6b15a11"),
		decodeHash("7959228bfdb316949973c08d8bb7bea2a21227a7b4ed85c35d247bf3d6b15a11"),
		decodeHash("6368616e676520746869732070617373776f726420746f206120736563726574"),
	}
	m = NewMerkleTree(inputs)
	rootHash := m.HashTree()
	rootHashHex := hex.EncodeToString(rootHash[:])
	assert.Equal(t, "4de26a6d1d6618f7bfeb3d168e37ef645db94c2d558bf8c3546d1311877ddffa", rootHashHex)
}
