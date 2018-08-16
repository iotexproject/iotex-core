// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"hash/fnv"
	"math/big"
	"math/rand"
	"testing"

	"github.com/facebookgo/clock"
	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestBlockDAO(t *testing.T) {
	getBlocks := func() []*Block {
		amount := uint64(50 << 22)
		// create testing transactions
		cbTsf1 := action.NewCoinBaseTransfer(big.NewInt(int64((amount))), testaddress.Addrinfo["alfa"].RawAddress)
		assert.NotNil(t, cbTsf1)
		cbTsf2 := action.NewCoinBaseTransfer(big.NewInt(int64((amount))), testaddress.Addrinfo["bravo"].RawAddress)
		assert.NotNil(t, cbTsf2)
		cbTsf3 := action.NewCoinBaseTransfer(big.NewInt(int64((amount))), testaddress.Addrinfo["charlie"].RawAddress)
		assert.NotNil(t, cbTsf3)

		hash1 := hash.Hash32B{}
		fnv.New32().Sum(hash1[:])
		blk1 := NewBlock(0, 1, hash1, clock.New(), []*action.Transfer{cbTsf1}, nil, nil)
		hash2 := hash.Hash32B{}
		fnv.New32().Sum(hash2[:])
		blk2 := NewBlock(0, 2, hash2, clock.New(), []*action.Transfer{cbTsf2}, nil, nil)
		hash3 := hash.Hash32B{}
		fnv.New32().Sum(hash3[:])
		blk3 := NewBlock(0, 3, hash3, clock.New(), []*action.Transfer{cbTsf3}, nil, nil)
		return []*Block{blk1, blk2, blk3}
	}

	blks := getBlocks()
	assert.Equal(t, 3, len(blks))

	testBlockDao := func(kvstore db.KVStore, t *testing.T) {
		ctx := context.Background()
		dao := newBlockDAO(kvstore)
		err := dao.Start(ctx)
		assert.Nil(t, err)
		defer func() {
			err = dao.Stop(ctx)
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
		assert.Equal(t, blks[0].Transfers[0].Hash(), blk.Transfers[0].Hash())
		height, err = dao.getBlockchainHeight()
		assert.Nil(t, err)
		assert.Equal(t, uint64(1), height)

		err = dao.putBlock(blks[2])
		assert.Nil(t, err)
		blk, err = dao.getBlock(blks[2].HashBlock())
		assert.Nil(t, err)
		assert.NotNil(t, blk)
		assert.Equal(t, blks[2].Transfers[0].Hash(), blk.Transfers[0].Hash())
		height, err = dao.getBlockchainHeight()
		assert.Nil(t, err)
		assert.Equal(t, uint64(3), height)

		err = dao.putBlock(blks[1])
		assert.Nil(t, err)
		blk, err = dao.getBlock(blks[1].HashBlock())
		assert.Nil(t, err)
		assert.NotNil(t, blk)
		assert.Equal(t, blks[1].Transfers[0].Hash(), blk.Transfers[0].Hash())
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
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testBlockDao(db.NewBoltDB(path, nil), t)
	})

}
