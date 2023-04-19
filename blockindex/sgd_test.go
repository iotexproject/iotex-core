package blockindex

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/stretchr/testify/require"
)

func TestSGDRegistry(t *testing.T) {
	require := require.New(t)
	kv := db.NewMemKVStore()
	indexer := NewSGDRegistry(kv, 0)
	ctx := context.Background()
	require.NoError(indexer.Start(ctx))
	defer func() {
		require.NoError(indexer.Stop(ctx))
	}()

	fakeTime := time.Now()

	t.Run("put block with no execution", func(t *testing.T) {
		tsf1, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(28), 1, big.NewInt(int64(1)), nil, testutil.TestGasLimit, big.NewInt(0))
		require.NoError(err)
		blk, err := block.NewTestingBuilder().
			AddActions(tsf1).
			SetHeight(1).
			SignAndBuild(identityset.PrivateKey(28))
		require.NoError(err)
		require.NoError(indexer.PutBlock(ctx, &blk))
		h, err := indexer.Height()
		require.NoError(err)
		require.Equal(uint64(1), h)
		idx, err := indexer.GetSGDIndex(identityset.Address(28).String())
		require.ErrorContains(err, "not exist in DB")
		require.Nil(idx)
	})

	t.Run("put block with deploy contract execution", func(t *testing.T) {
		execution1, err := action.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(28), 1, big.NewInt(1), 0, big.NewInt(0), nil)
		require.NoError(err)
		hash1, _ := execution1.Hash()
		receipt1 := &action.Receipt{
			Status:          uint64(1),
			ContractAddress: identityset.Address(31).String(),
			ActionHash:      hash1,
		}
		blk, err := block.NewTestingBuilder().
			AddActions(execution1).
			SetReceipts([]*action.Receipt{receipt1}).
			SetHeight(2).
			SetTimeStamp(fakeTime).
			SignAndBuild(identityset.PrivateKey(28))
		require.NoError(err)
		require.NoError(indexer.PutBlock(ctx, &blk))
		height, err := indexer.Height()
		require.NoError(err)
		require.Equal(uint64(2), height)
		idx, err := indexer.GetSGDIndex(identityset.Address(31).String())
		require.NoError(err)
		require.NotNil(idx)
		require.Equal(uint64(fakeTime.Unix()), idx.CreateTime)
		require.Equal(identityset.Address(28).String(), idx.Deployer)
		require.Equal(identityset.Address(31).String(), idx.Contract)
		require.Equal("", idx.Receiver)
		require.Equal(int32(0), idx.CallTimes)
	})

	t.Run("put block with contract call execution", func(t *testing.T) {
		execution2, err := action.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(28), 1, big.NewInt(1), 0, big.NewInt(0), nil)
		require.NoError(err)
		hash2, _ := execution2.Hash()
		receipt2 := &action.Receipt{
			Status:          uint64(1),
			ContractAddress: "",
			ActionHash:      hash2,
		}
		blk, err := block.NewTestingBuilder().
			AddActions(execution2).
			SetReceipts([]*action.Receipt{receipt2}).
			SetHeight(3).
			SetTimeStamp(fakeTime).
			SignAndBuild(identityset.PrivateKey(28))
		require.NoError(err)
		require.NoError(indexer.PutBlock(ctx, &blk))
		height, err := indexer.Height()
		require.NoError(err)
		require.Equal(uint64(3), height)
		idx, err := indexer.GetSGDIndex(identityset.Address(31).String())
		require.NoError(err)
		require.NotNil(idx)
		require.Equal(uint64(fakeTime.Unix()), idx.CreateTime)
		require.Equal(identityset.Address(28).String(), idx.Deployer)
		require.Equal(identityset.Address(31).String(), idx.Contract)
		require.Equal("", idx.Receiver)
		require.Equal(int32(1), idx.CallTimes)
	})

	t.Run("CheckContract", func(t *testing.T) {
		addr, percentage, ok := indexer.CheckContract(identityset.Address(31).String())
		require.True(ok)
		require.Equal(nil, addr)
		require.Equal(uint64(0), percentage)
	})

	t.Run("test DeleteTipBlock", func(t *testing.T) {
		require.ErrorContains(indexer.DeleteTipBlock(ctx, nil), "cannot remove")
	})
}
