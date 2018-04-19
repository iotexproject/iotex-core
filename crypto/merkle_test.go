// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package crypto

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func decodeHash(in string) [32]byte {
	hash, _ := hex.DecodeString(in)
	var arr [32]byte
	copy(arr[:], hash[:32])
	return arr
}

func TestMerkleTree(t *testing.T) {
	var inputs []Hash32B

	// with no leave
	m := NewMerkleTree(inputs)
	assert.Nil(t, m)

	// with one leave
	inputs = append(inputs, decodeHash("aeedd06eb44f08abbcc72a2293aff580f13662fa59cc1b0aa4a15ee7c118e4eb"))
	m = NewMerkleTree(inputs)
	expected := decodeHash("aeedd06eb44f08abbcc72a2293aff580f13662fa59cc1b0aa4a15ee7c118e4eb")
	actual1 := m.HashTree()
	assert.Equal(t, 0, bytes.Compare(expected[:], actual1[:]))

	// with two leaves
	inputs = append(inputs, decodeHash("9de6306b08158c423330f7a27243a1a5cbe39bfd764f07818437882d21241567"))
	m = NewMerkleTree(inputs)
	expected = decodeHash("0bbcd13e801fdf4d70c62b3788173edaf52113e7bfca4603eb5d486f4a011411")
	actual2 := m.HashTree()
	assert.Equal(t, 0, bytes.Compare(expected[:], actual2[:]))
	assert.Equal(t, -1, bytes.Compare(actual2[:], actual1[:]))

	// with three leaves
	inputs = append(inputs, decodeHash("7959228bfdb316949973c08d8bb7bea2a21227a7b4ed85c35d247bf3d6b15a11"))
	m = NewMerkleTree(inputs)
	expected = decodeHash("94ecbea840734c55eaac296c10d3a11715c3a2081f5bd0323d80d8ede5db029a")
	actual3 := m.HashTree()
	assert.Equal(t, 0, bytes.Compare(expected[:], actual3[:]))
	assert.Equal(t, 1, bytes.Compare(actual3[:], actual2[:]))

	// with four leaves, where 3rd leave == 4th leave
	inputs = append(inputs, decodeHash("7959228bfdb316949973c08d8bb7bea2a21227a7b4ed85c35d247bf3d6b15a11"))
	m = NewMerkleTree(inputs)
	expected = decodeHash("94ecbea840734c55eaac296c10d3a11715c3a2081f5bd0323d80d8ede5db029a")
	actual4 := m.HashTree()
	assert.Equal(t, 0, bytes.Compare(expected[:], actual4[:]))
	assert.Equal(t, 0, bytes.Compare(actual4[:], actual3[:]))

	// with five leaves, where 3rd leave == 4th leave
	inputs = append(inputs, decodeHash("6368616e676520746869732070617373776f726420746f206120736563726574"))
	m = NewMerkleTree(inputs)
	expected = decodeHash("6bef5fe8bccd931a37bb0453f71a277ffb8651daf4e4ce08a6f75a1621ecbf5a")
	actual5 := m.HashTree()
	assert.Equal(t, 0, bytes.Compare(expected[:], actual5[:]))
	assert.Equal(t, -1, bytes.Compare(actual5[:], actual4[:]))
}
