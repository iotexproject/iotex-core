// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"hash/fnv"
	"testing"

	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/stretchr/testify/assert"
)

func TestBlockDAO(t *testing.T) {
	getBlocks := func() []*Block {
		amount := uint64(50 << 22)
		// create testing transactions
		cbtx1 := NewCoinbaseTx(testaddress.Addrinfo["alfa"].Address, amount, GenesisCoinbaseData)
		cbtx2 := NewCoinbaseTx(testaddress.Addrinfo["bravo"].Address, amount, GenesisCoinbaseData)
		cbtx3 := NewCoinbaseTx(testaddress.Addrinfo["charlie"].Address, amount, GenesisCoinbaseData)

		hash1 := crypto.Hash32B{}
		fnv.New32().Sum(hash1[:])
		blk1 := NewBlock(0, 1, hash1, []*Tx{cbtx1})
		hash2 := crypto.Hash32B{}
		fnv.New32().Sum(hash2[:])
		blk2 := NewBlock(0, 2, hash2, []*Tx{cbtx2})
		hash3 := crypto.Hash32B{}
		fnv.New32().Sum(hash3[:])
		blk3 := NewBlock(0, 3, hash3, []*Tx{cbtx3})
		return []*Block{blk1, blk2, blk3}
	}

	blks := getBlocks()
	assert.Equal(t, 3, len(blks))

	dao := newBlockDAO(db.NewMemKVStore())
	err := dao.Init()
	assert.Nil(t, err)
	err = dao.Start()
	assert.Nil(t, err)
	defer func() {
		err = dao.Stop()
		assert.Nil(t, err)
	}()

	height, err := dao.getBlockchainHeight()
	assert.Nil(t, err)
	assert.Equal(t, uint32(0), height)

	// block put order is 0 2 1
	err = dao.putBlock(blks[0])
	assert.Nil(t, err)
	blk, err := dao.getBlock(blks[0].HashBlock())
	assert.Nil(t, err)
	assert.NotNil(t, blk)
	assert.Equal(t, blks[0].Tranxs[0].Hash(), blk.Tranxs[0].Hash())
	height, err = dao.getBlockchainHeight()
	assert.Nil(t, err)
	assert.Equal(t, uint32(1), height)

	err = dao.putBlock(blks[2])
	assert.Nil(t, err)
	blk, err = dao.getBlock(blks[2].HashBlock())
	assert.Nil(t, err)
	assert.NotNil(t, blk)
	assert.Equal(t, blks[2].Tranxs[0].Hash(), blk.Tranxs[0].Hash())
	height, err = dao.getBlockchainHeight()
	assert.Nil(t, err)
	assert.Equal(t, uint32(3), height)

	err = dao.putBlock(blks[1])
	assert.Nil(t, err)
	blk, err = dao.getBlock(blks[1].HashBlock())
	assert.Nil(t, err)
	assert.NotNil(t, blk)
	assert.Equal(t, blks[1].Tranxs[0].Hash(), blk.Tranxs[0].Hash())
	height, err = dao.getBlockchainHeight()
	assert.Nil(t, err)
	assert.Equal(t, uint32(3), height)

	// test getting hash by height
	hash, err := dao.getBlockHash(1)
	assert.Nil(t, err)
	assert.Equal(t, blks[0].HashBlock(), hash)

	hash, err = dao.getBlockHash(2)
	assert.Nil(t, err)
	assert.Equal(t, blks[1].HashBlock(), hash)

	hash, err = dao.getBlockHash(3)
	assert.Nil(t, err)
	assert.Equal(t, blks[2].HashBlock(), hash)

	// test getting height by hash
	height, err = dao.getBlockHeight(blks[0].HashBlock())
	assert.Nil(t, err)
	assert.Equal(t, blks[0].Height(), height)

	height, err = dao.getBlockHeight(blks[1].HashBlock())
	assert.Nil(t, err)
	assert.Equal(t, blks[1].Height(), height)

	height, err = dao.getBlockHeight(blks[2].HashBlock())
	assert.Nil(t, err)
	assert.Equal(t, blks[2].Height(), height)
}
