package staking

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

func TestBucketGuardKey(t *testing.T) {
	r := require.New(t)
	key := bucketGuardKey(0x0102030405060708)
	r.Equal(byte(9), key[0])
	r.Len(key, 9)
	r.Equal([]byte{1, 2, 3, 4, 5, 6, 7, 8}, key[1:])
}

func TestRequireUnguarded(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	bsm := NewBucketGuardStateManager(sm)

	r.NoError(requireUnguarded(sm, 1))

	pub := bytes.Repeat([]byte{0x22}, bucketguard.PubKeySize)
	r.NoError(bsm.Put(1, &BucketGuard{
		AlgID:       bucketguard.AlgSLHDSA192s,
		PublicKey:   pub,
		Generation:  1,
		Nonce:       2,
		BoundHeight: 3,
	}))
	err := requireUnguarded(sm, 1)
	r.Error(err)
	var receiptErr ReceiptError
	r.ErrorAs(err, &receiptErr)
	r.EqualValues(216, receiptErr.ReceiptStatus())

	err = requireUnguardedScope(sm, 1)
	r.Error(err)
	r.ErrorAs(err, &receiptErr)
	r.EqualValues(220, receiptErr.ReceiptStatus())

	r.NoError(bsm.Put(1, &BucketGuard{Generation: 1, Nonce: 3}))
	r.NoError(requireUnguarded(sm, 1))
}

func TestGuardedLegacyUnstakeRejected(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm, p, buckets, _ := initTestState(t, ctrl, []*bucketConfig{
		{identityset.Address(1), identityset.Address(1), "100000000000000000000", 30, false, false, nil, 0},
	}, []*candidateConfig{
		{identityset.Address(1), identityset.Address(1), identityset.Address(1), "test"},
	})
	bucket := buckets[0]
	r.NoError(NewBucketGuardStateManager(sm).Put(bucket.Index, &BucketGuard{
		AlgID:       bucketguard.AlgSLHDSA192s,
		PublicKey:   bytes.Repeat([]byte{0x22}, bucketguard.PubKeySize),
		Generation:  1,
		Nonce:       2,
		BoundHeight: 3,
	}))
	r.NoError(setupAccount(sm, identityset.Address(1), 100))

	act := action.NewUnstake(bucket.Index, nil)
	intrinsicGas, err := act.IntrinsicGas()
	r.NoError(err)
	elp := builder.SetNonce(1).SetGasLimit(10000).
		SetGasPrice(testGasPrice).SetAction(act).Build()

	ctx := genesis.WithGenesisContext(context.Background(), genesis.TestDefault())
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight:    1,
		BlockTimeStamp: bucket.StakeStartTime.Add(bucket.StakedDuration),
	})
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
	ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
		Caller:       identityset.Address(1),
		GasPrice:     testGasPrice,
		IntrinsicGas: intrinsicGas,
		Nonce:        1,
	})

	receipt, err := p.Handle(ctx, elp, sm)
	r.NoError(err)
	r.EqualValues(216, receipt.Status)
	r.False(bucket.isUnstaked())
}

func TestBucketGuardStateManager(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	bsm := NewBucketGuardStateManager(sm)
	bsr := NewBucketGuardStateReader(sm)

	got, err := bsr.Get(1)
	r.NoError(err)
	r.Nil(got)

	pub := bytes.Repeat([]byte{0x22}, bucketguard.PubKeySize)
	g := &BucketGuard{
		AlgID:       bucketguard.AlgSLHDSA192s,
		PublicKey:   pub,
		Generation:  1,
		Nonce:       2,
		BoundHeight: 3,
	}
	r.NoError(bsm.Put(1, g))
	got, err = bsr.Get(1)
	r.NoError(err)
	r.NotNil(got)
	r.True(got.Guarded)
	r.Equal(bucketguard.AlgSLHDSA192s, got.AlgID)
	r.Equal(pub, got.PublicKey)
	r.EqualValues(1, got.Generation)
	r.EqualValues(2, got.Nonce)
	r.EqualValues(3, got.BoundHeight)

	tombstone := &BucketGuard{Generation: 1, Nonce: 3}
	r.NoError(bsm.Put(1, tombstone))
	got, err = bsr.Get(1)
	r.NoError(err)
	r.NotNil(got)
	r.False(got.Guarded)
	r.EqualValues(1, got.Generation)
	r.EqualValues(3, got.Nonce)
	r.EqualValues(0, got.BoundHeight)

	r.NoError(bsm.Delete(1))
	got, err = bsr.Get(1)
	r.NoError(err)
	r.Nil(got)

	r.Error(bsm.Put(2, nil))
}
