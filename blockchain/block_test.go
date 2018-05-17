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
	cbtx0 := NewCoinbaseTx(ta.Addrinfo["miner"].RawAddress, amount, genesisCoinbaseData)
	assert.NotNil(cbtx0)
	cbtx1 := NewCoinbaseTx(ta.Addrinfo["alfa"].RawAddress, amount, genesisCoinbaseData)
	assert.NotNil(cbtx1)
	cbtx2 := NewCoinbaseTx(ta.Addrinfo["bravo"].RawAddress, amount, genesisCoinbaseData)
	assert.NotNil(cbtx2)
	cbtx3 := NewCoinbaseTx(ta.Addrinfo["charlie"].RawAddress, amount, genesisCoinbaseData)
	assert.NotNil(cbtx3)
	cbtx4 := NewCoinbaseTx(ta.Addrinfo["echo"].RawAddress, amount, genesisCoinbaseData)
	assert.NotNil(cbtx4)

	// verify tx hash
	hash0, _ := hex.DecodeString("ae8c129a18d50300a1b9ba9ba3e4b434457d13aa1892274d63515d913877e493")
	actual := cbtx0.Hash()
	assert.Equal(hash0, actual[:])
	t.Logf("actual hash = %x", actual[:])

	hash1, _ := hex.DecodeString("c2effdda1b2d3a11c51397b2a3506fe1a045a144881dea4521a9ccbb428f0a35")
	actual = cbtx1.Hash()
	assert.Equal(hash1, actual[:])
	t.Logf("actual hash = %x", actual[:])

	hash2, _ := hex.DecodeString("f6b283371222eca20f0aa970b47da5687b9df6836aabdca808b99bacdc937530")
	actual = cbtx2.Hash()
	assert.Equal(hash2, actual[:])
	t.Logf("actual hash = %x", actual[:])

	hash3, _ := hex.DecodeString("afdfb7f6f89da65d19c138cd5f83f6c53817a6372327809fd1637adcd2b3846e")
	actual = cbtx3.Hash()
	assert.Equal(hash3, actual[:])
	t.Logf("actual hash = %x", actual[:])

	hash4, _ := hex.DecodeString("c6ab82cbad37061c7e2936faa9fc117fc78c2d5daf45cbcc437a77eaa964e621")
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
