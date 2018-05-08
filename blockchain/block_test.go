// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/common"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
)

func TestBasicHash(t *testing.T) {
	assert := assert.New(t)

	// basic hash test
	input := []byte("hello")
	hash := sha256.Sum256(input)
	hash = sha256.Sum256(hash[:])
	hello, _ := hex.DecodeString("9595c9df90075148eb06860365df33584b75bff782a510c6cd4883a419833d50")
	assert.Equal(hello, hash[:])
	t.Logf("sha256(sha256(\"hello\") = %x", hash)

	hash = blake2b.Sum256(input)
	hash = blake2b.Sum256(hash[:])
	hello, _ = hex.DecodeString("901c60ffffd77f743729f8fea0233c0b00223428b5192c2015f853562b45ce59")
	assert.Equal(hello, hash[:])
	t.Logf("blake2b(blake2b(\"hello\") = %x", hash)
}

func TestMerkle(t *testing.T) {
	assert := assert.New(t)

	amount := uint64(50 << 22)
	// create testing transactions
	cbtx0 := NewCoinbaseTx(ta.Addrinfo["miner"].RawAddress, amount, GenesisCoinbaseData)
	assert.NotNil(cbtx0)
	cbtx1 := NewCoinbaseTx(ta.Addrinfo["alfa"].RawAddress, amount, GenesisCoinbaseData)
	assert.NotNil(cbtx1)
	cbtx2 := NewCoinbaseTx(ta.Addrinfo["bravo"].RawAddress, amount, GenesisCoinbaseData)
	assert.NotNil(cbtx2)
	cbtx3 := NewCoinbaseTx(ta.Addrinfo["charlie"].RawAddress, amount, GenesisCoinbaseData)
	assert.NotNil(cbtx3)
	cbtx4 := NewCoinbaseTx(ta.Addrinfo["echo"].RawAddress, amount, GenesisCoinbaseData)
	assert.NotNil(cbtx4)

	// verify tx hash
	hash0, _ := hex.DecodeString("02073334afb2b4502f0ecc5c73647e87890a55b854174d9e43d9ff5b1e21ff5f")
	actual := cbtx0.Hash()
	assert.Equal(hash0, actual[:])
	t.Logf("actual hash = %x", actual[:])

	hash1, _ := hex.DecodeString("e2d186144ffb1fe4e65af3cc7350048b77ba6b48d047bb4c0d78c3e6712b6767")
	actual = cbtx1.Hash()
	assert.Equal(hash1, actual[:])
	t.Logf("actual hash = %x", actual[:])

	hash2, _ := hex.DecodeString("3df7990d342ed67b4e00a34e82b4ec46655d39b42875c979ba8f0073f7be4968")
	actual = cbtx2.Hash()
	assert.Equal(hash2, actual[:])
	t.Logf("actual hash = %x", actual[:])

	hash3, _ := hex.DecodeString("3f7c369b5fdc2f4925f1f630d702e969f83d1a534788c3be71757ba9c39997f6")
	actual = cbtx3.Hash()
	assert.Equal(hash3, actual[:])
	t.Logf("actual hash = %x", actual[:])

	hash4, _ := hex.DecodeString("b71d8a9850483087be96f05f6353d3d96bc2cf85e54a8dac5a8f44b7dd614c7c")
	actual = cbtx4.Hash()
	assert.Equal(hash4, actual[:])
	t.Logf("actual hash = %x", actual[:])

	// manually compute merkle root
	cat := append(hash0, hash1...)
	hash01 := blake2b.Sum256(cat)
	t.Logf("hash01 = %x", hash01)

	cat = append(hash2, hash3...)
	hash23 := blake2b.Sum256(cat)
	t.Logf("hash23 = %x", hash23)

	cat = append(hash4, hash4...)
	hash45 := blake2b.Sum256(cat)
	t.Logf("hash45 = %x", hash45)

	cat = append(hash01[:], hash23[:]...)
	hash03 := blake2b.Sum256(cat)
	t.Logf("hash03 = %x", hash03)

	cat = append(hash45[:], hash45[:]...)
	hash47 := blake2b.Sum256(cat)
	t.Logf("hash47 = %x", hash47)

	cat = append(hash03[:], hash47[:]...)
	hash07 := blake2b.Sum256(cat)
	t.Logf("hash07 = %x", hash07)

	// create block using above 5 tx and verify merkle
	block := NewBlock(0, 0, common.ZeroHash32B, []*Tx{cbtx0, cbtx1, cbtx2, cbtx3, cbtx4})
	hash := block.TxRoot()
	assert.Equal(hash07[:], hash[:])
	t.Log("Merkle root match pass\n")

	// serialize
}
