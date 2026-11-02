// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/mohae/deepcopy"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto/signer"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

const testGuardChainID = 2

// guardTestCtx returns a context at height 10 and blkTime with every fork up
// to QSB active when qsb is true, and with QSB inactive otherwise.
func guardTestCtx(t *testing.T, qsb bool, blkTime time.Time, caller address.Address) context.Context {
	g := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
	g.ToBeEnabledBlockHeight = 1
	testutil.NormalizeGenesisHeights(&g.Blockchain)
	if !qsb {
		g.ToBeEnabledBlockHeight = 1_000_000
	}
	ctx := genesis.WithGenesisContext(context.Background(), g)
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{ChainID: testGuardChainID})
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: 10, BlockTimeStamp: blkTime})
	ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{Caller: caller})
	ctx = protocol.WithFeatureCtx(ctx)
	require.Equal(t, qsb, protocol.MustGetFeatureCtx(ctx).EnableBucketGuard)
	return ctx
}

func testGuardKey(t *testing.T, fill byte) *signer.PrivateKey {
	k, err := signer.NewKeyFromSeeds(pqcrypto.AlgSLHDSA192s, bytes.Repeat([]byte{fill}, signer.SeedSize(pqcrypto.AlgSLHDSA192s)))
	require.NoError(t, err)
	return k
}

// guardBucket stores a guard record for bucket index with key k.
func guardBucket(t *testing.T, sm protocol.StateManager, index uint64, k *signer.PrivateKey, generation, nonce uint64) *bucketguard.Guard {
	g := &bucketguard.Guard{AlgID: k.PublicKey().Alg(), PublicKey: k.PublicKey().Bytes(), Generation: generation, Nonce: nonce, BoundHeight: 1}
	require.NoError(t, putBucketGuard(sm, index, g))
	return g
}

// signGuardOp fills op's counters and deadline from cur and signs its CALL
// with k for a bucket owned by owner.
func signGuardOp(ctx context.Context, t *testing.T, k *signer.PrivateKey, owner address.Address, cur *bucketguard.Guard, op *bucketguard.GuardedBucketOp) *bucketguard.GuardedBucketOp {
	op.Generation, op.Nonce, op.Deadline = cur.Generation, cur.Nonce, 100
	sig, err := k.Sign([]byte(bucketguard.CallContext), bucketguard.EncodeCall(guardDomain(ctx), owner, op))
	require.NoError(t, err)
	op.Sig = sig
	return op
}

// grantFor verifies a CALL signed by k for op and returns its grant.
func grantFor(ctx context.Context, t *testing.T, k *signer.PrivateKey, owner address.Address, cur *bucketguard.Guard, op *bucketguard.GuardedBucketOp) *bucketguard.Grant {
	op = signGuardOp(ctx, t, k, owner, cur, op)
	grant, err := bucketguard.VerifyOp(bucketguard.ParamsV1(), guardDomain(ctx), owner, cur, op)
	require.NoError(t, err)
	return grant
}

func requireGuarded(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	r, ok := err.(ReceiptError)
	require.True(t, ok, "want a receipt error, got %v", err)
	require.Equal(t, uint64(iotextypes.ReceiptStatus_ErrBucketGuarded), r.ReceiptStatus(), err.Error())
}

// TestBucketWriteAuthorization covers the IIP-65 §10 checks in the candidate
// state manager: bucket writes and deletions, self-stake references and
// endorsements of a guarded bucket.
func TestBucketWriteAuthorization(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	owner1, owner2 := identityset.Address(1), identityset.Address(2)
	buckets := []*bucketConfig{
		{owner1, owner2, "1200000000000000000000000", 30, false, false, nil, 0},
		{owner1, owner2, "1200000000000000000000000", 30, false, false, nil, 0},
	}
	cands := []*candidateConfig{{owner1, identityset.Address(7), owner1, "test1"}}
	k := testGuardKey(t, 1)
	ctx := guardTestCtx(t, true, timeBlock, owner2)

	newState := func() (protocol.StateManager, CandidateStateManager, *bucketguard.Guard) {
		sm, _, _, _ := initTestStateWithHeight(t, ctrl, buckets, cands, 10)
		cur := guardBucket(t, sm, 0, k, 1, 0)
		csm, err := NewCandidateStateManagerWithContext(ctx, sm)
		r.NoError(err)
		return sm, csm, cur
	}
	bucket := func(csm CandidateStateManager, index uint64) *VoteBucket {
		b, err := csm.NativeBucket(index)
		r.NoError(err)
		return b
	}

	t.Run("legacy authority may only add stake", func(t *testing.T) {
		_, csm, _ := newState()
		b := bucket(csm, 0)
		b.StakedAmount.Add(b.StakedAmount, big.NewInt(1))
		r.NoError(csm.updateBucket(0, b, legacyAuth))

		b = bucket(csm, 0)
		b.StakedAmount.Sub(b.StakedAmount, big.NewInt(2))
		requireGuarded(t, csm.updateBucket(0, b, legacyAuth))
		b = bucket(csm, 0)
		b.Owner = identityset.Address(3)
		requireGuarded(t, csm.updateBucket(0, b, legacyAuth))
		requireGuarded(t, csm.delBucketAndIndex(owner2, owner1, 0, legacyAuth))

		// bucket 1 is not guarded
		b = bucket(csm, 1)
		b.Owner = identityset.Address(3)
		r.NoError(csm.updateBucket(1, b, legacyAuth))
	})

	t.Run("a grant covers its own bucket, counters and fields", func(t *testing.T) {
		sm, csm, cur := newState()
		unstake := grantFor(ctx, t, k, owner2, cur, &bucketguard.GuardedBucketOp{BucketIndex: 0, Op: bucketguard.OpUnstake})

		b := bucket(csm, 0)
		b.Owner = identityset.Address(3)
		requireGuarded(t, csm.updateBucket(0, b, guardAuth(unstake)))
		requireGuarded(t, csm.delBucketAndIndex(owner2, owner1, 0, guardAuth(unstake)))
		b = bucket(csm, 0)
		b.UnstakeStartTime = timeBlock
		r.NoError(csm.updateBucket(0, b, guardAuth(unstake)))

		// once the counters move, the grant no longer covers the bucket
		guardBucket(t, sm, 0, k, 1, 1)
		b = bucket(csm, 0)
		b.UnstakeStartTime = timeBlock.Add(time.Hour)
		requireGuarded(t, csm.updateBucket(0, b, guardAuth(unstake)))
	})

	t.Run("a withdraw grant deletes its bucket only", func(t *testing.T) {
		sm, csm, cur := newState()
		guardBucket(t, sm, 1, k, 1, 0)
		withdraw := grantFor(ctx, t, k, owner2, cur, &bucketguard.GuardedBucketOp{BucketIndex: 0, Op: bucketguard.OpWithdraw, Recipient: owner2.Bytes()})
		requireGuarded(t, csm.delBucketAndIndex(owner2, owner1, 1, guardAuth(withdraw)))
		r.NoError(csm.delBucketAndIndex(owner2, owner1, 0, guardAuth(withdraw)))
	})

	t.Run("a guarded bucket cannot become a self-stake bucket", func(t *testing.T) {
		_, csm, _ := newState()
		c := csm.GetByName("test1")
		c.SelfStakeBucketIdx = 0
		requireGuarded(t, csm.Upsert(c))
		c.SelfStakeBucketIdx = 1
		r.NoError(csm.Upsert(c))
	})

	t.Run("a guarded bucket cannot be endorsed", func(t *testing.T) {
		sm, _, _ := newState()
		requireGuarded(t, NewEndorsementStateManager(sm).enforceBucketGuard(true).Put(0, &Endorsement{ExpireHeight: 100}))
		r.NoError(NewEndorsementStateManager(sm).enforceBucketGuard(true).Put(1, &Endorsement{ExpireHeight: 100}))
	})

	t.Run("nothing is enforced before the fork", func(t *testing.T) {
		sm, _, _, _ := initTestStateWithHeight(t, ctrl, buckets, cands, 10)
		guardBucket(t, sm, 0, k, 1, 0)
		csm, err := NewCandidateStateManagerWithContext(guardTestCtx(t, false, timeBlock, owner2), sm)
		r.NoError(err)
		b := bucket(csm, 0)
		b.Owner = identityset.Address(3)
		r.NoError(csm.updateBucket(0, b, legacyAuth))
		r.NoError(NewEndorsementStateManager(sm).Put(0, &Endorsement{ExpireHeight: 100}))
	})
}

// TestGuardedOpVerifiesBeforeWriting checks IIP-65 §9.2: a guarded op whose
// signature fails writes nothing at all, even before the staking snapshot is
// reverted, and the same op signed by the guard succeeds.
func TestGuardedOpVerifiesBeforeWriting(t *testing.T) {
	owner1, owner2 := identityset.Address(1), identityset.Address(2)
	fresh := identityset.Address(25) // no account yet
	cands := []*candidateConfig{
		{owner1, identityset.Address(7), owner1, "test1"},
		{owner2, identityset.Address(8), owner2, "test2"},
	}
	guardKey, otherKey := testGuardKey(t, 1), testGuardKey(t, 2)
	for _, tc := range []struct {
		name   string
		bucket *bucketConfig
		op     *bucketguard.GuardedBucketOp
	}{
		{"unstake", &bucketConfig{owner1, owner2, "1200000000000000000000000", 30, false, false, nil, 0},
			&bucketguard.GuardedBucketOp{Op: bucketguard.OpUnstake}},
		{"withdraw to a new account", &bucketConfig{owner1, owner2, "1200000000000000000000000", 30, false, false, &timeBeforeBlockII, 0},
			&bucketguard.GuardedBucketOp{Op: bucketguard.OpWithdraw, Recipient: fresh.Bytes()}},
		{"transfer", &bucketConfig{owner1, owner2, "1200000000000000000000000", 30, true, false, nil, 0},
			&bucketguard.GuardedBucketOp{Op: bucketguard.OpTransfer, NewOwner: fresh.Bytes(), ClearGuard: true}},
		{"change candidate", &bucketConfig{owner1, owner2, "1200000000000000000000000", 30, true, false, nil, 0},
			&bucketguard.GuardedBucketOp{Op: bucketguard.OpChangeCandidate, CandidateName: "test2", ExpectedCandidateID: owner2.Bytes()}},
		{"restake", &bucketConfig{owner1, owner2, "1200000000000000000000000", 30, true, false, nil, 0},
			&bucketguard.GuardedBucketOp{Op: bucketguard.OpRestake, Duration: 60, AutoStake: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			ctrl := gomock.NewController(t)
			ctx := guardTestCtx(t, true, timeBlock, identityset.Address(3))
			run := func(k *signer.PrivateKey) (error, int, protocol.StateManager, CandidateStateManager, map[string]string) {
				sm, p, _, _ := initTestStateWithHeight(t, ctrl, []*bucketConfig{tc.bucket}, cands, 10)
				cur := guardBucket(t, sm, 0, guardKey, 1, 0)
				counted := &writeCountingSM{StateManager: sm}
				csm, err := NewCandidateStateManagerWithContext(ctx, counted)
				r.NoError(err)
				b, err := csm.NativeBucket(0)
				r.NoError(err)
				r.NoError(csm.DebitBucketPool(b.StakedAmount, true))
				before := dumpStakingState(t, csm, sm)
				counted.writes = 0
				op := *tc.op
				_, _, err = p.handleGuardedBucketOp(ctx, action.NewGuardedBucketOp(signGuardOp(ctx, t, k, owner2, cur, &op)), csm)
				return err, counted.writes, sm, csm, before
			}

			err, writes, sm, csm, before := run(otherKey)
			re, ok := err.(ReceiptError)
			r.True(ok, "want a receipt error, got %v", err)
			r.Equal(uint64(iotextypes.ReceiptStatus_ErrGuardSignature), re.ReceiptStatus())
			r.Zero(writes, "nothing may be written before the signature is verified")
			r.Equal(before, dumpStakingState(t, csm, sm))

			err, writes, sm, _, _ = run(guardKey)
			r.NoError(err)
			r.NotZero(writes)
			g, err := getBucketGuard(sm, 0)
			r.NoError(err)
			if tc.op.Op == bucketguard.OpWithdraw {
				r.Nil(g, "withdraw deletes the guard record")
			} else {
				r.Equal(uint64(1), g.Nonce)
			}
		})
	}
}

// TestBucketGuardRecordOutsideVoterScans stores guard records next to the
// voter indexes the IIP-59 reward settlement scans. With _bucketGuard = 6 the
// record of bucket 1 fell inside the contract-staking owner index range and
// ScanFrozenVoters failed with "unexpected key", failing the block.
func TestBucketGuardRecordOutsideVoterScans(t *testing.T) {
	r := require.New(t)
	ctx := forkGateCtx(eraTestFreezeHeight, true)
	sm := eraTestSM(t)
	csm := eraTestCSM(ctx, sm)
	cs := contractstaking.NewContractStakingStateManager(sm)
	cand := identityset.Address(1)
	nativeVoter := eraTestAddress(t, 0x40, 1)
	contractVoter := eraTestAddress(t, 0x80, 1)

	// Buckets 0, 1 and 2; guard records at index 0 would sort below the scan
	// start, so the records go on 1 and 2.
	for i := uint64(0); i < 3; i++ {
		idx, err := csm.putBucketAndIndex(NewVoteBucket(cand, nativeVoter, big.NewInt(100), 91, time.Now(), true))
		r.NoError(err)
		r.Equal(i, idx)
	}
	r.NoError(cs.UpsertBucket(ctx, identityset.Address(20), 1, &contractstaking.Bucket{
		Owner: contractVoter, Candidate: cand, StakedAmount: big.NewInt(100),
	}))
	r.NoError(BeginEraCOWWindow(ctx, sm, eraTestFreezeHeight))
	window, err := LoadEraCOWWindow(sm)
	r.NoError(err)

	guardBucket(t, sm, 1, testGuardKey(t, 1), 1, 0)
	// A cleared guard leaves a tombstone, which is a record all the same.
	r.NoError(putBucketGuard(sm, 2, &bucketguard.Guard{Generation: 2, Nonce: 3, BoundHeight: 5}))

	for _, limits := range [][2]int{{0, 0}, {1, 100}, {10, 4}} {
		var (
			voters []address.Address
			resume []byte
		)
		for {
			page, err := ScanFrozenVoters(sm, window, make([]byte, _addressLength), nil, resume, limits[0], limits[1])
			r.NoError(err, "limits %v", limits)
			voters = append(voters, page.Voters...)
			if page.Complete {
				break
			}
			r.NotEqual(resume, page.ResumeAfter, "limits %v: scan made no progress", limits)
			resume = page.ResumeAfter
		}
		r.Equal([]address.Address{nativeVoter, contractVoter}, voters, "limits %v", limits)
	}
}
