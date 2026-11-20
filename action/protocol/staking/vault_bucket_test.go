package staking

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
	"go.uber.org/mock/gomock"
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

func TestVaultBucketRecords(t *testing.T) {
	r := require.New(t)
	r.Equal(byte(10), _vaultBucket)
	r.Equal(byte(11), _vaultGuard)
	r.Equal(byte(12), _vaultMeta)
	r.Equal("0a0102030405060708", hex.EncodeToString(vaultKey(_vaultBucket, 0x0102030405060708)))
	type record interface {
		systemcontracts.GenericValueContainer
		Serialize() ([]byte, error)
		Deserialize([]byte) error
	}
	for _, tc := range []struct {
		name  string
		value record
		fresh func() record
	}{
		{"vault", &VaultBucketRecord{bucketguard.VaultBucket{VaultIndex: 1, Owner: bytes.Repeat([]byte{7}, 20), Amount: []byte{0xff}}}, func() record { return &VaultBucketRecord{} }},
		{"guard", &BucketGuardV2Record{bucketguard.BucketGuardV2{AlgID: pqcrypto.AlgSLHDSA192s, PublicKey: bytes.Repeat([]byte{8}, 48), BucketClass: bucketguard.ClassAVault, Generation: 1, GuardNonce: 3, NextRequestID: 2, BoundHeight: 4, UnbindDelay: bucketguard.UnbindDelayBlocks, CancelCooldown: 9, Pending: &bucketguard.PendingUnbind{RequestID: 1, StartHeight: 4, ExpiresAt: 4 + bucketguard.UnbindDelayBlocks}}}, func() record { return &BucketGuardV2Record{} }},
		{"tombstone", &BucketGuardV2Record{bucketguard.BucketGuardV2{BucketClass: bucketguard.ClassAVault, Generation: 2, GuardNonce: 4, NextRequestID: 2, BoundHeight: 100}}, func() record { return &BucketGuardV2Record{} }},
		{"meta zero", &VaultMetaRecord{}, func() record { return &VaultMetaRecord{} }},
		{"meta max", &VaultMetaRecord{TotalVaultCount: math.MaxUint64}, func() record { return &VaultMetaRecord{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			b, err := tc.value.Serialize()
			r.NoError(err)
			got := tc.fresh()
			r.NoError(got.Deserialize(b))
			r.Equal(tc.value, got)
			v, err := tc.value.Encode()
			r.NoError(err)
			r.Equal(b, v.PrimaryData)
			got = tc.fresh()
			r.NoError(got.Decode(v))
			r.Equal(tc.value, got)
			// Unknown fields, repeated fields and truncation may never rewrite
			// a previously valid object, even when decoding a corrupt backend.
			bad := [][]byte{append(bytes.Clone(b), 0x78, 1), {0x80}}
			if len(b) != 0 {
				bad = append(bad, append(bytes.Clone(b), b...), b[:len(b)-1])
			}
			for _, raw := range bad {
				r.Error(got.Deserialize(raw))
				r.Equal(tc.value, got)
			}
		})
	}
}

func TestVaultMetaCanonical(t *testing.T) {
	for _, b := range [][]byte{{8, 0}, {8, 0x81, 0}, {0x88, 0, 1}, {10, 0}, {8, 1, 8, 1}} {
		v := VaultMetaRecord{TotalVaultCount: 17}
		require.Error(t, v.Deserialize(b))
		require.Equal(t, uint64(17), v.TotalVaultCount)
	}
}

// TestVaultBucketHandlerBoundaries runs production handlers against the mock-backed
// StateManager at the exact consensus heights, without shortening the delay.
func TestVaultBucketHandlerBoundaries(t *testing.T) {
	ctrl := gomock.NewController(t)
	owner := identityset.Address(2)
	sm, p, _, _ := initTestStateWithHeight(t, ctrl, []*bucketConfig{{identityset.Address(1), owner, "1200000000000000000000000", 30, true, false, nil, 0}}, []*candidateConfig{{identityset.Address(1), identityset.Address(7), identityset.Address(1), "test1"}}, 10)
	testdb.AllowRevert(sm.(*mock_chainmanager.MockStateManager))
	funded, err := accountutil.LoadAccount(sm, owner)
	require.NoError(t, err)
	require.NoError(t, funded.AddBalance(unit.ConvertIotxToRau(10000)))
	require.NoError(t, accountutil.StoreAccount(sm, owner, funded))
	ctx := guardTestCtx(t, true, timeBlock, owner)
	ac := protocol.MustGetActionCtx(ctx)
	ac.GasPrice = new(big.Int)
	ac.Nonce = 1
	ctx = protocol.WithActionCtx(ctx, ac)
	csm, err := NewCandidateStateManagerWithContext(ctx, sm)
	require.NoError(t, err)
	k := testGuardKey(t, 0x66)
	pk := k.PublicKey().Bytes()
	run := func(m *bucketguard.VaultAction) error {
		a, err := action.NewVaultAction(m)
		require.NoError(t, err)
		_, _, err = p.handleVaultAction(ctx, a, csm)
		return err
	}
	create := &bucketguard.VaultAction{Method: "vaultCreate", Amount: unit.ConvertIotxToRau(500), OwnerTxNonce: 1, Guard: bucketguard.NewGuard{AlgID: pqcrypto.AlgSLHDSA192s, PublicKey: pk}}
	create.Guard.AcceptSig, err = k.Sign([]byte(bucketguard.VaultAcceptContext), bucketguard.EncodeVaultAccept(guardDomain(ctx), 0, 1, 1, owner.Bytes(), bucketguard.UnbindDelayBlocks, pqcrypto.AlgSLHDSA192s, pk, [64]byte{}))
	require.NoError(t, err)
	before, err := accountutil.LoadAccount(sm, owner)
	require.NoError(t, err)
	before = before.Clone()
	require.NoError(t, run(create))
	pool := bucketguard.VaultPool()
	poolAddr, err := address.FromBytes(pool[:])
	require.NoError(t, err)
	account, err := accountutil.LoadAccount(sm, poolAddr)
	require.NoError(t, err)
	require.Equal(t, create.Amount, account.Balance)
	require.NoError(t, run(&bucketguard.VaultAction{Method: "vaultInitiateExit"}))
	guard, err := ReadVaultGuard(sm, 0)
	require.NoError(t, err)
	expiry := guard.Pending.ExpiresAt
	at := func(h uint64) {
		bc := protocol.MustGetBlockCtx(ctx)
		bc.BlockHeight = h
		ctx = protocol.WithBlockCtx(ctx, bc)
	}
	at(expiry - 1)
	require.Error(t, run(&bucketguard.VaultAction{Method: "vaultFinalizeExit"}))
	require.NoError(t, run(&bucketguard.VaultAction{Method: "vaultCancelExit"}))
	require.NoError(t, run(&bucketguard.VaultAction{Method: "vaultInitiateExit"}))
	guard, err = ReadVaultGuard(sm, 0)
	require.NoError(t, err)
	expiry = guard.Pending.ExpiresAt
	at(expiry)
	require.Error(t, run(&bucketguard.VaultAction{Method: "vaultCancelExit"}))
	require.NoError(t, run(&bucketguard.VaultAction{Method: "vaultFinalizeExit"}))
	after, err := accountutil.LoadAccount(sm, owner)
	require.NoError(t, err)
	require.Equal(t, before.Balance, after.Balance)
	account, err = accountutil.LoadAccount(sm, poolAddr)
	require.NoError(t, err)
	require.Zero(t, account.Balance.Sign())
	_, err = ReadVaultBucket(sm, 0)
	require.ErrorIs(t, err, state.ErrStateNotExist)
	guard, err = ReadVaultGuard(sm, 0)
	require.NoError(t, err)
	require.Empty(t, guard.PublicKey)
	require.Error(t, run(&bucketguard.VaultAction{Method: "vaultFinalizeExit"}))
	// A recovery-enabled real staking bucket freezes only automatic deposits.
	at(10)
	bind := &bucketguard.VaultAction{Method: "bindBucketGuardV2", UnbindDelay: bucketguard.UnbindDelayBlocks, Guard: bucketguard.NewGuard{AlgID: pqcrypto.AlgSLHDSA192s, PublicKey: pk}}
	bind.Guard.AcceptSig, err = k.Sign([]byte(bucketguard.AcceptContext), bucketguard.EncodeAcceptBV2(guardDomain(ctx), 0, 1, owner.Bytes(), bind.UnbindDelay, pqcrypto.AlgSLHDSA192s, pk, [64]byte{}))
	require.NoError(t, err)
	require.NoError(t, run(bind))
	require.NoError(t, run(&bucketguard.VaultAction{Method: "guardUnbindInitiate"}))
	bg, err := ReadStakingGuardV2(sm, 0)
	require.NoError(t, err)
	expiry = bg.Pending.ExpiresAt
	bucket, err := csm.NativeBucket(0)
	require.NoError(t, err)
	principal := new(big.Int).Set(bucket.StakedAmount)
	require.ErrorIs(t, p.AddDepositForCompound(ctx, sm, owner, 0, big.NewInt(123), FrozenSelfStake{}), ErrCompoundSkippedPendingUnbind)
	bucket, err = csm.NativeBucket(0)
	require.NoError(t, err)
	require.Equal(t, principal, bucket.StakedAmount)
	bg2, err := ReadStakingGuardV2(sm, 0)
	require.NoError(t, err)
	require.Equal(t, bg, bg2)
	cancel := func(id uint64) *bucketguard.VaultAction {
		bg, err := ReadStakingGuardV2(sm, 0)
		require.NoError(t, err)
		height := protocol.MustGetBlockCtx(ctx).BlockHeight
		anchor := height - 1
		h, err := testBlockHash(anchor)
		require.NoError(t, err)
		m := &bucketguard.VaultAction{Method: "guardUnbindCancel", RequestID: id, Call: bucketguard.VaultCall{Generation: bg.Generation, Deadline: height + 10, AnchorHeight: anchor, AnchorHash: h[:]}}
		m.Call.Sig, err = k.Sign([]byte(bucketguard.GuardCancelContext), bucketguard.EncodeGuardCancel(guardDomain(ctx), &bucketguard.GuardCancel{Generation: bg.Generation, RequestID: id, Deadline: m.Call.Deadline, AnchorHeight: anchor, AnchorHash: h[:]}))
		require.NoError(t, err)
		return m
	}
	at(expiry - 1)
	require.NoError(t, run(cancel(1)))
	bg, err = ReadStakingGuardV2(sm, 0)
	require.NoError(t, err)
	require.Nil(t, bg.Pending)
	require.Equal(t, expiry-1+bucketguard.CancelCooldownBlocks, bg.CancelCooldown)
	require.NoError(t, run(cancel(1)), "cleared request retries are idempotent")
	require.ErrorIs(t, run(&bucketguard.VaultAction{Method: "guardUnbindInitiate"}), bucketguard.ErrCancelCooldown)
	at(bg.CancelCooldown)
	require.NoError(t, run(&bucketguard.VaultAction{Method: "guardUnbindInitiate"}))
	require.ErrorIs(t, run(cancel(1)), bucketguard.ErrStaleRequestId)
	bg, err = ReadStakingGuardV2(sm, 0)
	require.NoError(t, err)
	at(bg.Pending.ExpiresAt)
	require.NoError(t, run(&bucketguard.VaultAction{Method: "guardUnbindFinalize"}))
	old, err := getBucketGuard(sm, 0)
	require.NoError(t, err)
	require.False(t, old.IsGuarded())
}

func TestDeleteBucketGuardWithBucketV2(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, active := range []bool{false, true} {
			t.Run(fmt.Sprintf("legacy=%t/active=%t", legacy, active), func(t *testing.T) {
				ctrl := gomock.NewController(t)
				sm := testdb.NewMockStateManager(ctrl)
				ctx := guardTestCtx(t, true, timeBlock, identityset.Address(2))
				if legacy {
					require.NoError(t, putBucketGuard(sm, 0, &bucketguard.Guard{Generation: 2}))
				}
				guard := &BucketGuardV2Record{BucketGuardV2: bucketguard.BucketGuardV2{BucketClass: bucketguard.ClassBStaking, Generation: 2, GuardNonce: 7, NextRequestID: 1, BoundHeight: 1}}
				if active {
					guard.AlgID = pqcrypto.AlgSLHDSA192s
					guard.PublicKey = bytes.Repeat([]byte{3}, 48)
					guard.UnbindDelay = bucketguard.UnbindDelayBlocks
				}
				_, err := sm.PutState(guard, vaultStateOptions(vaultKey(_stakingGuardV2, 0))...)
				require.NoError(t, err)
				tombstone := &BucketGuardV2Record{BucketGuardV2: bucketguard.BucketGuardV2{BucketClass: bucketguard.ClassAVault, Generation: 5, GuardNonce: 8, NextRequestID: 1, BoundHeight: 1}}
				_, err = sm.PutState(tombstone, vaultStateOptions(vaultKey(_vaultGuard, 0))...)
				require.NoError(t, err)
				preCtx := guardTestCtx(t, false, timeBlock, identityset.Address(2))
				require.NoError(t, deleteBucketGuardWithBucket(preCtx, sm, 0))
				got, err := ReadStakingGuardV2(sm, 0)
				require.NoError(t, err)
				require.Equal(t, guard, got)
				require.NoError(t, deleteBucketGuardWithBucket(ctx, sm, 0))
				_, err = ReadStakingGuardV2(sm, 0)
				require.ErrorIs(t, err, state.ErrStateNotExist)
				old, err := getBucketGuard(sm, 0)
				require.NoError(t, err)
				require.Nil(t, old)
				got, err = ReadVaultGuard(sm, 0)
				require.NoError(t, err)
				require.Equal(t, tombstone, got)
				require.NoError(t, deleteBucketGuardWithBucket(ctx, sm, 0), "absent records are harmless")
			})
		}
	}
}
