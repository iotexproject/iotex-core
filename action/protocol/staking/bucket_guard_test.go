package staking

import (
	"bytes"
	"context"
	"testing"
	"time"

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

func TestGuardedLegacyPaths(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm, p, buckets, _ := initTestState(t, ctrl, []*bucketConfig{
		{identityset.Address(1), identityset.Address(1), "100000000000000000000", 30, false, false, nil, 0},
		{identityset.Address(1), identityset.Address(1), "1200000000000000000000000", 30, false, true, nil, 0},
	}, []*candidateConfig{
		{identityset.Address(1), identityset.Address(1), identityset.Address(1), "test"},
	})
	bsm := NewBucketGuardStateManager(sm)
	pub := bytes.Repeat([]byte{0x22}, bucketguard.PubKeySize)
	for _, b := range buckets {
		r.NoError(bsm.Put(b.Index, &BucketGuard{
			AlgID:       bucketguard.AlgSLHDSA192s,
			PublicKey:   pub,
			Generation:  1,
			Nonce:       2,
			BoundHeight: 3,
		}))
	}
	r.NoError(setupAccount(sm, identityset.Address(1), 100))
	csm, err := NewCandidateStateManagerWithContext(context.Background(), sm)
	r.NoError(err)

	ctx := genesis.WithGenesisContext(context.Background(), genesis.TestDefault())
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight:    1,
		BlockTimeStamp: buckets[0].StakeStartTime.Add(buckets[0].StakedDuration),
	})
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
	ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
		Caller:   identityset.Address(1),
		GasPrice: testGasPrice,
		Nonce:    1,
	})

	transfer, err := action.NewTransferStake(identityset.Address(2).String(), 0, nil)
	r.NoError(err)
	ownership, err := action.NewCandidateTransferOwnership(identityset.Address(2).String(), nil)
	r.NoError(err)
	migrate := builder.SetNonce(1).SetGasLimit(10000).
		SetGasPrice(testGasPrice).SetAction(action.NewMigrateStake(0)).Build()

	tests := []struct {
		name   string
		status uint64
		call   func() error
	}{
		{"unstake", 216, func() error {
			_, err := p.handleUnstake(ctx, action.NewUnstake(0, nil), csm)
			return err
		}},
		{"withdraw", 216, func() error {
			_, _, err := p.handleWithdrawStake(ctx, action.NewWithdrawStake(0, nil), csm)
			return err
		}},
		{"change candidate", 216, func() error {
			_, err := p.handleChangeCandidate(ctx, action.NewChangeCandidate("test", 0, nil), csm)
			return err
		}},
		{"transfer", 216, func() error {
			_, err := p.handleTransferStake(ctx, transfer, csm)
			return err
		}},
		{"restake", 216, func() error {
			_, err := p.handleRestake(ctx, action.NewRestake(0, 30, false, nil), csm)
			return err
		}},
		{"migrate", 216, func() error {
			_, _, _, _, err := p.handleStakeMigrate(ctx, migrate, csm)
			return err
		}},
		{"candidate activate", 216, func() error {
			_, _, err := p.handleCandidateActivate(ctx, action.NewCandidateActivate(0), csm)
			return err
		}},
		{"candidate endorsement", 216, func() error {
			_, _, err := p.handleCandidateEndorsement(ctx, action.NewCandidateEndorsementLegacy(0, true), csm)
			return err
		}},
		{"candidate deactivate", 220, func() error {
			_, _, err := p.handleCandidateDeactivate(ctx, action.NewCandidateDeactivate(action.CandidateDeactivateOpRequest), csm)
			return err
		}},
		{"candidate transfer ownership", 220, func() error {
			_, _, err := p.handleCandidateTransferOwnership(ctx, ownership, csm)
			return err
		}},
		{"schedule candidate deactivation", 220, func() error {
			_, _, err := p.handleScheduleCandidateDeactivation(ctx, action.NewScheduleCandidateDeactivation(identityset.Address(1)), csm)
			return err
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			r.Error(err)
			var receiptErr ReceiptError
			r.ErrorAs(err, &receiptErr)
			r.Equal(tc.status, receiptErr.ReceiptStatus())
		})
	}
}

func TestGuardedDepositAllowed(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm, p, buckets, _ := initTestState(t, ctrl, []*bucketConfig{
		{identityset.Address(1), identityset.Address(1), "100000000000000000000", 30, true, false, nil, 0},
	}, []*candidateConfig{
		{identityset.Address(1), identityset.Address(1), identityset.Address(1), "test"},
	})
	guard := &BucketGuard{
		Guarded:     true,
		AlgID:       bucketguard.AlgSLHDSA192s,
		PublicKey:   bytes.Repeat([]byte{0x22}, bucketguard.PubKeySize),
		Generation:  1,
		Nonce:       2,
		BoundHeight: 3,
	}
	bsm := NewBucketGuardStateManager(sm)
	r.NoError(bsm.Put(buckets[0].Index, guard))
	r.NoError(setupAccount(sm, identityset.Address(1), 100))
	csm, err := NewCandidateStateManagerWithContext(context.Background(), sm)
	r.NoError(err)

	act, err := action.NewDepositToStake(buckets[0].Index, "1000000000000000000", nil)
	r.NoError(err)
	ctx := genesis.WithGenesisContext(context.Background(), genesis.TestDefault())
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight:    1,
		BlockTimeStamp: buckets[0].StakeStartTime.Add(time.Hour),
	})
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
	ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
		Caller:   identityset.Address(1),
		GasPrice: testGasPrice,
		Nonce:    1,
	})
	_, _, err = p.handleDepositToStake(ctx, act, csm)
	r.NoError(err)

	got, err := NewBucketGuardStateReader(sm).Get(buckets[0].Index)
	r.NoError(err)
	r.Equal(guard, got)
}
