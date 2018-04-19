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

	cp "github.com/iotexproject/iotex-core/crypto"
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
	cbtx0 := NewCoinbaseTx(ta.Addrinfo["miner"].Address, amount, GenesisCoinbaseData)
	cbtx1 := NewCoinbaseTx(ta.Addrinfo["alfa"].Address, amount, GenesisCoinbaseData)
	cbtx2 := NewCoinbaseTx(ta.Addrinfo["bravo"].Address, amount, GenesisCoinbaseData)
	cbtx3 := NewCoinbaseTx(ta.Addrinfo["charlie"].Address, amount, GenesisCoinbaseData)
	cbtx4 := NewCoinbaseTx(ta.Addrinfo["echo"].Address, amount, GenesisCoinbaseData)

	// verify tx hash
	hash0, _ := hex.DecodeString("90e0967d54b5f6f898c95404d0818f3f7a332ee6d5d7439666dd1e724771cb5e")
	actual := cbtx0.Hash()
	assert.Equal(hash0, actual[:])

	hash1, _ := hex.DecodeString("7959228bfdb316949973c08d8bb7bea2a21227a7b4ed85c35d247bf3d6b15a11")
	actual = cbtx1.Hash()
	assert.Equal(hash1, actual[:])

	hash2, _ := hex.DecodeString("b57c9d659f99c21c38ab3323996ad69cbb5f417fbce496be25d35b3ebecefca6")
	actual = cbtx2.Hash()
	assert.Equal(hash2, actual[:])

	hash3, _ := hex.DecodeString("e5f326657b615336301ccaed5c5f6e2952547d7bdf7ee198458fd7e9e2e2b3f7")
	actual = cbtx3.Hash()
	assert.Equal(hash3, actual[:])

	hash4, _ := hex.DecodeString("f937d7a934e25230ec3c701b18ce108f322519e70544745ac3f848739f4efa80")
	actual = cbtx4.Hash()
	assert.Equal(hash4, actual[:])

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
	block := NewBlock(0, 0, cp.ZeroHash32B, []*Tx{cbtx0, cbtx1, cbtx2, cbtx3, cbtx4})
	hash := block.MerkleRoot()
	assert.Equal(hash07[:], hash[:])
	t.Log("Merkle root match pass\n")

	// serialize
}
