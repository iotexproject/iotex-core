package blockchain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestProposalPool(t *testing.T) {
	r := require.New(t)
	signer := identityset.PrivateKey(1)
	blockInterval := 5 * time.Second
	mint := func(prev *block.Block, round int) (*block.Block, error) {
		ra := block.NewRunnableActionsBuilder().Build()
		blk, err := block.NewBuilder(ra).
			SetPrevBlockHash(prev.HashBlock()).
			SetHeight(prev.Height() + 1).
			SetTimestamp(prev.Timestamp().Add(blockInterval * time.Duration(round+1))).
			SignAndBuild(signer)
		if err != nil {
			return nil, err
		}
		return &blk, nil
	}
	t.Run("addblocks", func(t *testing.T) {
		var blocks []*block.Block
		gb := block.GenesisBlock()
		prev := gb
		for i := 1; i < 10; i++ {
			blk, err := mint(prev, 0)
			r.NoError(err)
			blocks = append(blocks, blk)
			prev = blk
		}
		pool := newProposalPool()
		for i := range blocks {
			blk := blocks[i]
			pool.AddBlock(blk)
			got := pool.Block(blk.Height())
			r.Equal(blk, got, "height %d", blk.Height())
			r.Equal(blk.Height(), pool.Tip(), "height %d", blk.Height())
		}
	})
	t.Run("forks", func(t *testing.T) {
		var blocks []*block.Block
		gb := block.GenesisBlock()
		for i := 0; i < 5; i++ {
			prev := gb
			blk, err := mint(prev, i)
			r.NoError(err)
			blocks = append(blocks, blk)
			blk, err = mint(blk, 0)
			r.NoError(err)
			blocks = append(blocks, blk)
		}
		pool := newProposalPool()
		for i := range blocks {
			blk := blocks[i]
			pool.AddBlock(blk)
			got := pool.Block(blk.Height())
			r.Equal(blk, got, "index %d", i)
			r.Equal(blk.Timestamp(), pool.forks[blk.HashBlock()])
		}
		expectForks := map[hash.Hash256]time.Time{
			blocks[1].HashBlock(): blocks[1].Timestamp(),
			blocks[3].HashBlock(): blocks[3].Timestamp(),
			blocks[5].HashBlock(): blocks[5].Timestamp(),
			blocks[7].HashBlock(): blocks[7].Timestamp(),
		}
		for f, t := range expectForks {
			r.Equal(t, pool.forks[f])
		}
		confirm := blocks[6]
		r.Equal(uint64(1), confirm.Height())
		r.NoError(pool.ReceiveBlock(confirm))
		r.Nil(pool.Block(0))
		r.Nil(pool.Block(1))
		r.Equal(blocks[7], pool.Block(2))
	})
}
