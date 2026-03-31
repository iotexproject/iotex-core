package actpool

import (
	"context"
	"math/big"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func newTestBundle(t *testing.T, nonce uint64, targetHeight uint64) *action.Bundle {
	t.Helper()
	se, err := action.SignedTransfer(
		identityset.Address(1).String(),
		identityset.PrivateKey(2),
		nonce,
		big.NewInt(1),
		nil,
		21000,
		big.NewInt(1),
	)
	require.NoError(t, err)
	b := action.NewBundle()
	require.NoError(t, b.Add(se))
	b.SetTargetBlockHeight(targetHeight)
	return b
}

func newTestBlock(t *testing.T, height uint64) block.Block {
	t.Helper()
	blk, err := block.NewTestingBuilder().SetHeight(height).SignAndBuild(identityset.PrivateKey(0))
	require.NoError(t, err)
	return blk
}

func TestBundlePool_ReceiveBlock(t *testing.T) {
	ctx := context.Background()
	sender := identityset.Address(2)

	t.Run("nil block", func(t *testing.T) {
		bp := NewBundlePool(genesis.Default)
		require.ErrorIs(t, bp.ReceiveBlock(nil), ErrNilBlock)
	})

	t.Run("height too low", func(t *testing.T) {
		bp := NewBundlePool(genesis.Default)
		blk100 := newTestBlock(t, 100)
		require.NoError(t, bp.ReceiveBlock(&blk100))
		// same height
		blk100again := newTestBlock(t, 100)
		require.ErrorIs(t, bp.ReceiveBlock(&blk100again), ErrBlockHeightTooLow)
		// lower height
		blk99 := newTestBlock(t, 99)
		require.ErrorIs(t, bp.ReceiveBlock(&blk99), ErrBlockHeightTooLow)
	})

	t.Run("removes bundles at target height only", func(t *testing.T) {
		// After the first block is received (height > 0), only bundles whose
		// targetHeight == received block height are removed; future bundles survive.
		bp := NewBundlePool(genesis.Default)
		blk100 := newTestBlock(t, 100)
		require.NoError(t, bp.ReceiveBlock(&blk100))

		require.NoError(t, bp.AddBundle(ctx, sender, "uuid-101", newTestBundle(t, 1, 101)))
		require.NoError(t, bp.AddBundle(ctx, sender, "uuid-102", newTestBundle(t, 2, 102)))
		require.Equal(t, 2, bp.Size())

		blk101 := newTestBlock(t, 101)
		require.NoError(t, bp.ReceiveBlock(&blk101))

		require.Equal(t, 1, bp.Size())
		_, _, bundles, err := bp.BundlesAtHeight(102)
		require.NoError(t, err)
		require.Equal(t, 1, len(bundles))
		_, _, _, err = bp.BundlesAtHeight(101)
		require.ErrorIs(t, err, ErrNoBundlesForHeight)
	})

	t.Run("initial state prunes stale bundles on first block", func(t *testing.T) {
		// When height == 0 (pool just created / after Reset), the first ReceiveBlock
		// prunes all bundles whose targetHeight <= received block height.
		bp := NewBundlePool(genesis.Default)

		require.NoError(t, bp.AddBundle(ctx, sender, "uuid-50", newTestBundle(t, 1, 50)))
		require.NoError(t, bp.AddBundle(ctx, sender, "uuid-90", newTestBundle(t, 2, 90)))
		require.Equal(t, 2, bp.Size())

		blk60 := newTestBlock(t, 60)
		require.NoError(t, bp.ReceiveBlock(&blk60))

		// height=50 bundle pruned (50 <= 60); height=90 bundle survives (90 > 60)
		require.Equal(t, 1, bp.Size())
		_, _, _, err := bp.BundlesAtHeight(50)
		require.ErrorIs(t, err, ErrNoBundlesForHeight)
		_, _, bundles, err := bp.BundlesAtHeight(90)
		require.NoError(t, err)
		require.Equal(t, 1, len(bundles))
	})

	t.Run("Reset clears pool and resets height", func(t *testing.T) {
		bp := NewBundlePool(genesis.Default)
		blk100 := newTestBlock(t, 100)
		require.NoError(t, bp.ReceiveBlock(&blk100))

		require.NoError(t, bp.AddBundle(ctx, sender, "uuid-101", newTestBundle(t, 1, 101)))
		require.Equal(t, 1, bp.Size())

		bp.Reset()
		require.Equal(t, 0, bp.Size())

		// After Reset height is 0; bundles for any height 1..maxLookahead are accepted again.
		require.NoError(t, bp.AddBundle(ctx, sender, "uuid-50-after-reset", newTestBundle(t, 2, 50)))
		require.Equal(t, 1, bp.Size())
	})
}

func TestBundlePool_Validator(t *testing.T) {
	ctx := context.Background()
	sender := identityset.Address(2)

	t.Run("validator rejects bundle", func(t *testing.T) {
		bp := NewBundlePool(genesis.Default)
		bp.SetValidator(func(_ context.Context, _ *action.SealedEnvelope) error {
			return errors.New("rejected by validator")
		})
		blk := newTestBlock(t, 100)
		require.NoError(t, bp.ReceiveBlock(&blk))

		err := bp.AddBundle(ctx, sender, "uuid-1", newTestBundle(t, 1, 101))
		require.Error(t, err)
		require.Equal(t, 0, bp.Size())
	})

	t.Run("validator passes bundle", func(t *testing.T) {
		bp := NewBundlePool(genesis.Default)
		bp.SetValidator(func(_ context.Context, _ *action.SealedEnvelope) error {
			return nil
		})
		blk := newTestBlock(t, 100)
		require.NoError(t, bp.ReceiveBlock(&blk))

		require.NoError(t, bp.AddBundle(ctx, sender, "uuid-1", newTestBundle(t, 1, 101)))
		require.Equal(t, 1, bp.Size())
	})

	t.Run("validator called for each action in bundle", func(t *testing.T) {
		bp := NewBundlePool(genesis.Default)
		callCount := 0
		bp.SetValidator(func(_ context.Context, _ *action.SealedEnvelope) error {
			callCount++
			return nil
		})
		blk := newTestBlock(t, 100)
		require.NoError(t, bp.ReceiveBlock(&blk))

		// Bundle with 2 actions
		se1 := newTestBundle(t, 1, 101)
		se2, err := action.SignedTransfer(
			identityset.Address(1).String(), identityset.PrivateKey(3),
			2, big.NewInt(1), nil, 21000, big.NewInt(1),
		)
		require.NoError(t, err)
		require.NoError(t, se1.Add(se2))

		require.NoError(t, bp.AddBundle(ctx, sender, "uuid-1", se1))
		require.Equal(t, 2, callCount)
	})
}

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
			// beyond max lookahead
			bundle.SetTargetBlockHeight(100 + DefaultMaxBundleLookahead + 1)
			require.ErrorIs(bp.AddBundle(ctx, sender, "uuid-far-future", bundle), ErrBundleTargetHeight)
			bundle.SetTargetBlockHeight(102)
		})
		t.Run("pool size limit", func(t *testing.T) {
			bp.SetMaxSize(1) // pool already has 1 bundle
			bundle2 := action.NewBundle()
			bundle2.Add(se)
			bundle2.SetTargetBlockHeight(103)
			require.ErrorIs(bp.AddBundle(ctx, sender, "uuid-overflow", bundle2), ErrBundlePoolFull)
			bp.SetMaxSize(DefaultMaxBundlePoolSize) // restore
		})
		t.Run("add dup bundle", func(t *testing.T) {
			require.Error(bp.AddBundle(ctx, sender, "uuid-dup-hash", bundle))
			bundle.SetTargetBlockHeight(103)
			require.Error(bp.AddBundle(ctx, sender, "uuid-valid", bundle))
			require.Equal(1, bp.Size())
			bundle.SetTargetBlockHeight(102)
		})
		t.Run("Delete bundle", func(t *testing.T) {
			require.NoError(bp.DeleteBundle("uuid-valid"))
			require.Equal(0, bp.Size())
			_, _, _, err := bp.BundlesAtHeight(102)
			require.Error(err)
		})
	})
}
