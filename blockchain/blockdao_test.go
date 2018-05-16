// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"hash/fnv"
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/common/utils"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestBlockDAO(t *testing.T) {
	getBlocks := func() []*Block {
		amount := uint64(50 << 22)
		// create testing transactions
		cbtx1 := NewCoinbaseTx(testaddress.Addrinfo["alfa"].RawAddress, amount, testCoinbaseData)
		assert.NotNil(t, cbtx1)
		cbtx2 := NewCoinbaseTx(testaddress.Addrinfo["bravo"].RawAddress, amount, testCoinbaseData)
		assert.NotNil(t, cbtx2)
		cbtx3 := NewCoinbaseTx(testaddress.Addrinfo["charlie"].RawAddress, amount, testCoinbaseData)
		assert.NotNil(t, cbtx3)

		hash1 := common.Hash32B{}
		fnv.New32().Sum(hash1[:])
		blk1 := NewBlock(0, 1, hash1, []*Tx{cbtx1})
		hash2 := common.Hash32B{}
		fnv.New32().Sum(hash2[:])
		blk2 := NewBlock(0, 2, hash2, []*Tx{cbtx2})
		hash3 := common.Hash32B{}
		fnv.New32().Sum(hash3[:])
		blk3 := NewBlock(0, 3, hash3, []*Tx{cbtx3})
		return []*Block{blk1, blk2, blk3}
	}

	blks := getBlocks()
	assert.Equal(t, 3, len(blks))

	testBlockDao := func(kvstore db.KVStore, t *testing.T) {
		dao := newBlockDAO(kvstore)
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
		assert.Equal(t, uint64(0), height)

		// block put order is 0 2 1
		err = dao.putBlock(blks[0])
		assert.Nil(t, err)
		blk, err := dao.getBlock(blks[0].HashBlock())
		assert.Nil(t, err)
		assert.NotNil(t, blk)
		assert.Equal(t, blks[0].Tranxs[0].Hash(), blk.Tranxs[0].Hash())
		height, err = dao.getBlockchainHeight()
		assert.Nil(t, err)
		assert.Equal(t, uint64(1), height)

		err = dao.putBlock(blks[2])
		assert.Nil(t, err)
		blk, err = dao.getBlock(blks[2].HashBlock())
		assert.Nil(t, err)
		assert.NotNil(t, blk)
		assert.Equal(t, blks[2].Tranxs[0].Hash(), blk.Tranxs[0].Hash())
		height, err = dao.getBlockchainHeight()
		assert.Nil(t, err)
		assert.Equal(t, uint64(3), height)

		err = dao.putBlock(blks[1])
		assert.Nil(t, err)
		blk, err = dao.getBlock(blks[1].HashBlock())
		assert.Nil(t, err)
		assert.NotNil(t, blk)
		assert.Equal(t, blks[1].Tranxs[0].Hash(), blk.Tranxs[0].Hash())
		height, err = dao.getBlockchainHeight()
		assert.Nil(t, err)
		assert.Equal(t, uint64(3), height)

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

	t.Run("In-memory KV Store", func(t *testing.T) {
		testBlockDao(db.NewMemKVStore(), t)
	})

	path := "/tmp/test-kv-store-" + string(rand.Int())
	t.Run("Bolt DB", func(t *testing.T) {
		cleanup := func() {
			if utils.FileExists(path) {
				err := os.Remove(path)
				assert.Nil(t, err)
			}
		}

		cleanup()
		defer cleanup()
		testBlockDao(db.NewBoltDB(path, nil), t)
	})

}
