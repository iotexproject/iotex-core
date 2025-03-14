package blockchain

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestBlockPreparer(t *testing.T) {
	r := require.New(t)

	bp := newBlockPreparer()
	prevHash := block.GenesisHash()

	// PrepareBlock: successful mint
	mintCount := 0
	bp.PrepareBlock(prevHash[:], func() (*block.Block, error) {
		mintCount++
		return new(block.Block), nil
	})

	blk, err := bp.WaitBlock(prevHash[:])
	r.NoError(err)
	r.NotNil(blk)
	r.Equal(1, mintCount)

	// PrepareBlock: repeated call shouldn't mint again
	bp.PrepareBlock(prevHash[:], func() (*block.Block, error) {
		mintCount++
		return new(block.Block), nil
	})
	bp.PrepareBlock(prevHash[:], func() (*block.Block, error) {
		mintCount++
		return new(block.Block), nil
	})

	blk, err = bp.WaitBlock(prevHash[:])
	r.NoError(err)
	r.NotNil(blk)
	r.Equal(2, mintCount, "no additional mint expected")

	// PrepareBlock: mint with error
	bp.PrepareBlock(prevHash[:], func() (*block.Block, error) {
		return nil, errors.New("mint error")
	})

	blk, err = bp.WaitBlock(prevHash[:])
	r.Error(err)
	r.Nil(blk)

	// ReceiveBlock
	testBlk, err := block.NewBuilder(block.NewRunnableActionsBuilder().Build()).SetPrevBlockHash(prevHash).SignAndBuild(identityset.PrivateKey(1))
	r.NoError(err)
	err = bp.ReceiveBlock(&testBlk)
	r.NoError(err)
}
