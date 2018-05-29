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

	trx "github.com/iotexproject/iotex-core/blockchain/trx"
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
	cbtx0 := trx.NewCoinbaseTx(ta.Addrinfo["miner"].RawAddress, amount, testCoinbaseData)
	assert.NotNil(cbtx0)
	cbtx1 := trx.NewCoinbaseTx(ta.Addrinfo["alfa"].RawAddress, amount, testCoinbaseData)
	assert.NotNil(cbtx1)
	cbtx2 := trx.NewCoinbaseTx(ta.Addrinfo["bravo"].RawAddress, amount, testCoinbaseData)
	assert.NotNil(cbtx2)
	cbtx3 := trx.NewCoinbaseTx(ta.Addrinfo["charlie"].RawAddress, amount, testCoinbaseData)
	assert.NotNil(cbtx3)
	cbtx4 := trx.NewCoinbaseTx(ta.Addrinfo["echo"].RawAddress, amount, testCoinbaseData)
	assert.NotNil(cbtx4)

	// verify tx hash
	hash0, _ := hex.DecodeString("1a0130efd104f6236dac0f1c8c6817465f082e5555c29ef8878abb5ddc8c6002")
	actual := cbtx0.Hash()
	assert.Equal(hash0, actual[:])
	t.Logf("actual hash = %x", actual[:])

	hash1, _ := hex.DecodeString("bc469f01661dfeb6c1309b5545cddf12836fee8eaa101f0233879089399d73af")
	actual = cbtx1.Hash()
	assert.Equal(hash1, actual[:])
	t.Logf("actual hash = %x", actual[:])

	hash2, _ := hex.DecodeString("4edc136bb713dae6d077a37c8110103334391a24bfcce6dffc8934d1dd24b51d")
	actual = cbtx2.Hash()
	assert.Equal(hash2, actual[:])
	t.Logf("actual hash = %x", actual[:])

	hash3, _ := hex.DecodeString("762404453347e2cdb87239a689eb30645dbef9935a775b58652b44c6e2ce8282")
	actual = cbtx3.Hash()
	assert.Equal(hash3, actual[:])
	t.Logf("actual hash = %x", actual[:])

	hash4, _ := hex.DecodeString("1019ce698e2ac013a5a8d24c67a33e333cf21f1f27ea2986636f2e44c3bbf5e1")
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
	block := NewBlock(0, 0, common.ZeroHash32B, []*trx.Tx{cbtx0, cbtx1, cbtx2, cbtx3, cbtx4})
	hash := block.TxRoot()
	assert.Equal(hash07[:], hash[:])
	t.Log("Merkle root match pass\n")

	// serialize
}
