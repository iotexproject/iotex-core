package rolldpos

import (
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestProposalPool(t *testing.T) {
	r := require.New(t)
	t.Run("empty", func(t *testing.T) {
		pool := newProposalPool()
		// create a mock block
		rootHash := hash.ZeroHash256
		blk := makeBlock2(r, 1, rootHash)
		// set pool root
		pool.Init(rootHash)
		err := pool.AddBlock(blk)
		r.NoError(err)
		r.NotNil(pool.BlockByHash(blk.HashBlock()))
		r.Contains(pool.leaves, blk.HashBlock())
	})
	t.Run("already-exist", func(t *testing.T) {
		pool := newProposalPool()
		rootHash := hash.ZeroHash256
		blk := makeBlock2(r, 1, rootHash)
		pool.Init(rootHash)
		err := pool.AddBlock(blk)
		r.NoError(err)
		// add again
		err2 := pool.AddBlock(blk)
		r.NoError(err2, "should not error when block already exists")
	})
	t.Run("new-tip-block", func(t *testing.T) {
		pool := newProposalPool()
		rootHash := hash.ZeroHash256
		parentBlk := makeBlock2(r, 1, rootHash)
		pool.Init(rootHash)
		r.NoError(pool.AddBlock(parentBlk))

		// add child
		childBlk := makeBlock2(r, 2, parentBlk.HashBlock())
		err := pool.AddBlock(childBlk)
		r.NoError(err)
		r.NotNil(pool.BlockByHash(childBlk.HashBlock()))
		r.Contains(pool.leaves, childBlk.HashBlock())
		r.NotContains(pool.leaves, parentBlk.HashBlock(), "parent should be removed from leaves")
	})
	t.Run("new-fork", func(t *testing.T) {
		pool := newProposalPool()
		rootHash := hash.ZeroHash256
		blkA := makeBlock2(r, 1, rootHash)
		pool.Init(rootHash)
		r.NoError(pool.AddBlock(blkA))

		// root -> blkA
		//      -> blkB
		blkB := makeBlock2(r, 1, rootHash)
		r.NotEqual(blkA.HashBlock(), blkB.HashBlock())
		r.NoError(pool.AddBlock(blkB))
		r.NotNil(pool.BlockByHash(blkA.HashBlock()))
		r.Contains(pool.leaves, blkA.HashBlock())
		r.Contains(pool.leaves, blkB.HashBlock())

		// root -> blkA
		// 			-> blkAA
		//          -> blkAB
		//      -> blkB
		//          -> blkBA
		//          -> blkBB
		blkAA := makeBlock2(r, 2, blkA.HashBlock())
		r.NoError(pool.AddBlock(blkAA))
		r.NotNil(pool.BlockByHash(blkAA.HashBlock()))
		r.Contains(pool.leaves, blkAA.HashBlock())
		r.NotContains(pool.leaves, blkA.HashBlock(), "blkA should be removed from leaves")
		blkAB := makeBlock2(r, 2, blkA.HashBlock())
		r.NoError(pool.AddBlock(blkAB))
		r.NotNil(pool.BlockByHash(blkAB.HashBlock()))
		r.Contains(pool.leaves, blkAB.HashBlock())
		blkBA := makeBlock2(r, 2, blkB.HashBlock())
		r.NoError(pool.AddBlock(blkBA))
		r.NotNil(pool.BlockByHash(blkBA.HashBlock()))
		r.Contains(pool.leaves, blkBA.HashBlock())
		r.NotContains(pool.leaves, blkB.HashBlock(), "blkB should be removed from leaves")
		blkBB := makeBlock2(r, 2, blkB.HashBlock())
		r.NoError(pool.AddBlock(blkBB))
		r.NotNil(pool.BlockByHash(blkBB.HashBlock()))
		r.Contains(pool.leaves, blkBB.HashBlock())

		// blkA confirmed
		// root -> blkA -> blkAA
		//          	-> blkAB
		r.NoError(pool.ReceiveBlock(blkA))
		r.NotNil(pool.BlockByHash(blkAA.HashBlock()))
		r.NotNil(pool.BlockByHash(blkAB.HashBlock()))
		r.Nil(pool.BlockByHash(blkA.HashBlock()))
		r.Nil(pool.BlockByHash(blkB.HashBlock()))
		r.Nil(pool.BlockByHash(blkBA.HashBlock()))
		r.Nil(pool.BlockByHash(blkBB.HashBlock()))
	})
	t.Run("invalid-parent", func(t *testing.T) {
		pool := newProposalPool()
		rootHash := hash.ZeroHash256
		pool.Init(rootHash)
		parent := makeBlock2(r, 1, rootHash)
		blk := makeBlock2(r, 2, parent.HashBlock())
		err := pool.AddBlock(blk)
		r.Error(err)
		r.Contains(err.Error(), "is not a tip of any fork")
	})
}

// makeBlock is a helper to create a block with the given height and prevHash
func makeBlock2(r *require.Assertions, height uint64, prevHash hash.Hash256) *block.Block {
	// minimal mock
	bd := block.NewBuilder(block.NewRunnableActionsBuilder().Build()).
		SetPrevBlockHash(prevHash).
		SetHeight(height).
		SetTimestamp(time.Now())
	blk, err := bd.SignAndBuild(identityset.PrivateKey(1))
	r.NoError(err)
	return &blk
}
