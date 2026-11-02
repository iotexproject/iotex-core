// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"context"
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
