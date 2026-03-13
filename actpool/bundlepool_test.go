package actpool

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestBundlePool_AddAndGetBundles(t *testing.T) {
	require := require.New(t)
	bp := NewBundlePool(genesis.Default)
	blk, err := block.NewTestingBuilder().SetHeight(100).SignAndBuild(identityset.PrivateKey(0))
	require.NoError(err)
	se, err := action.SignedTransfer(
		identityset.Address(1).String(),
		identityset.PrivateKey(2),
		0,
		big.NewInt(1000),
		[]byte("test"),
		100000,
		big.NewInt(1),
	)
	require.NoError(err)
	se2, err := action.SignedTransfer(
		identityset.Address(2).String(),
		identityset.PrivateKey(3),
		0,
		big.NewInt(1000),
		[]byte("test"),
		100000,
		big.NewInt(1),
	)
	require.NoError(err)
	require.NoError(bp.ReceiveBlock(&blk))
	ctx := context.Background()
	sender := identityset.Address(2)
	t.Run("nil bundle", func(t *testing.T) {
		require.Error(bp.AddBundle(ctx, sender, "uuid-1", nil))
	})
	t.Run("empty bundle", func(t *testing.T) {
		require.Error(bp.AddBundle(ctx, sender, "uuid-empty", action.NewBundle()))
	})

	t.Run("non-empty bundle", func(t *testing.T) {
		bundle := action.NewBundle()
		bundle.Add(se)
		bundle.Add(se2)
		bundle.SetTargetBlockHeight(102)
		require.Equal(0, bp.Size())
		require.NoError(bp.AddBundle(ctx, sender, "uuid-valid", bundle))
		require.Equal(1, bp.Size())
		t.Run("get bundle by target height", func(t *testing.T) {
			uuids, senders, bundles, err := bp.BundlesAtHeight(102)
			require.NoError(err)
			require.Equal(1, len(bundles))
			require.Equal(1, len(uuids))
			require.Equal(1, len(senders))
			require.Equal("uuid-valid", uuids[0])
			require.Equal(sender.String(), senders[0].String())
			require.Equal(2, bundles[0].Len())
			require.Equal(uint64(102), bundles[0].TargetBlockHeight())
		})
		t.Run("invalid target height", func(t *testing.T) {
			bundle.SetTargetBlockHeight(99)
			require.Error(bp.AddBundle(ctx, sender, "uuid-less", bundle))
			bundle.SetTargetBlockHeight(100)
			require.Error(bp.AddBundle(ctx, sender, "uuid-equal", bundle))
		})
		t.Run("add dup bundle", func(t *testing.T) {
			require.Error(bp.AddBundle(ctx, sender, "uuid-dup-hash", bundle))
			bundle.SetTargetBlockHeight(103)
			require.Error(bp.AddBundle(ctx, sender, "uuid-valid", bundle))
			require.Equal(1, bp.Size())
			bundle.SetTargetBlockHeight(102)
		})
		t.Run("Delete bundle", func(t *testing.T) {
			require.NoError(bp.DeleteBundle(sender, "uuid-valid"))
			require.Equal(0, bp.Size())
			_, _, _, err := bp.BundlesAtHeight(102)
			require.Error(err)
		})
	})
}
