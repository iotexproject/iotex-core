// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"encoding/hex"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/mohae/deepcopy"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// writeCountingSM counts the writes made through it.
type writeCountingSM struct {
	protocol.StateManager
	writes int
}

func (sm *writeCountingSM) PutState(s interface{}, opts ...protocol.StateOption) (uint64, error) {
	sm.writes++
	return sm.StateManager.PutState(s, opts...)
}

func (sm *writeCountingSM) DelState(opts ...protocol.StateOption) (uint64, error) {
	sm.writes++
	return sm.StateManager.DelState(opts...)
}

// rawState captures the stored bytes of a state.
type rawState struct{ b []byte }

func (r *rawState) Deserialize(b []byte) error {
	r.b = append([]byte(nil), b...)
	return nil
}

// dumpStakingState returns every stored staking, candidate and account
// record, the candidates in csm's view and its bucket pool total.
func dumpStakingState(t *testing.T, csm CandidateStateManager, sm protocol.StateManager) map[string]string {
	out := map[string]string{}
	for _, ns := range []string{_stakingNameSpace, _candidateNameSpace, state.AccountKVNamespace} {
		_, iter, err := sm.States(protocol.NamespaceOption(ns))
		if errors.Cause(err) == state.ErrStateNotExist {
			continue
		}
		require.NoError(t, err)
		for i := 0; i < iter.Size(); i++ {
			var r rawState
			k, err := iter.Next(&r)
			require.NoError(t, err)
			out[ns+"/"+hex.EncodeToString(k)] = hex.EncodeToString(r.b)
		}
	}
	c := csm.(*candSM)
	cands := c.candCenter.All()
	sort.Slice(cands, func(i, j int) bool { return cands[i].GetIdentifier().String() < cands[j].GetIdentifier().String() })
	for _, d := range cands {
		b, err := d.Serialize()
		require.NoError(t, err)
		out["view/candidate/"+d.GetIdentifier().String()] = hex.EncodeToString(b)
	}
	out["view/pool"] = fmt.Sprintf("%s/%d", c.bucketPool.total.amount, c.bucketPool.total.count)
	return out
}

func receiptStatusOf(err error) string {
	if err == nil {
		return iotextypes.ReceiptStatus_Success.String()
	}
	if r, ok := err.(ReceiptError); ok {
		return iotextypes.ReceiptStatus(r.ReceiptStatus()).String()
	}
	return "non-receipt error: " + err.Error()
}

// TestBucketOpsMatchBeforeQSB runs each split operation (check, then apply)
// and its *BeforeQSB counterpart on identical states, with every fork up to
// QSB active. On success the resulting states must be equal. On failure the
// receipt statuses must be equal, and the split check must not have written
// anything.
func TestBucketOpsMatchBeforeQSB(t *testing.T) {
	var (
		owner1, owner2 = identityset.Address(1), identityset.Address(2)
		amount         = "1200000000000000000000000"
		early          = timeBeforeBlockI.Add(24 * time.Hour)
		recent         = timeBlock.Add(-time.Hour)
		cands          = []*candidateConfig{
			{owner1, identityset.Address(7), owner1, "test1"},
			{owner2, identityset.Address(8), owner2, "test2"},
		}
	)
	type run func(ctx context.Context, p *Protocol, csm CandidateStateManager, b *VoteBucket, split bool) error
	unstake := func(ctx context.Context, p *Protocol, csm CandidateStateManager, b *VoteBucket, split bool) error {
		if !split {
			return p.unstakeBucketBeforeQSB(ctx, csm, b.Index, b)
		}
		c, err := p.checkUnstake(ctx, csm, b.Index, b)
		if err != nil {
			return err
		}
		return c.apply(ctx, csm, legacyAuth)
	}
	withdraw := func(ctx context.Context, p *Protocol, csm CandidateStateManager, b *VoteBucket, split bool) error {
		acct, err := accountutil.LoadAccount(csm.SM(), b.Owner)
		if err != nil {
			return err
		}
		if !split {
			return p.withdrawStakedBucketBeforeQSB(ctx, csm, b.Index, b, b.Owner, acct)
		}
		c, err := p.checkWithdraw(ctx, csm, b.Index, b, b.Owner, acct)
		if err != nil {
			return err
		}
		return c.apply(ctx, csm, legacyAuth)
	}
	changeTo := func(name string) run {
		return func(ctx context.Context, p *Protocol, csm CandidateStateManager, b *VoteBucket, split bool) error {
			cand := csm.GetByName(name)
			if !split {
				return p.changeBucketCandidateBeforeQSB(ctx, csm, b.Index, b, cand)
			}
			c, err := p.checkChangeCandidate(ctx, csm, b.Index, b, cand)
			if err != nil {
				return err
			}
			return c.apply(ctx, csm, legacyAuth)
		}
	}
	transferTo := func(to address.Address) run {
		return func(ctx context.Context, p *Protocol, csm CandidateStateManager, b *VoteBucket, split bool) error {
			if !split {
				return p.transferBucketOwnerBeforeQSB(ctx, csm, b.Index, b, to)
			}
			c, err := p.checkTransfer(ctx, csm, b.Index, b, to)
			if err != nil {
				return err
			}
			return c.apply(ctx, csm, legacyAuth)
		}
	}
	restake := func(days uint32, autoStake bool) run {
		return func(ctx context.Context, p *Protocol, csm CandidateStateManager, b *VoteBucket, split bool) error {
			if !split {
				return p.restakeBucketBeforeQSB(ctx, csm, b.Index, b, days, autoStake)
			}
			c, err := p.checkRestake(ctx, csm, b.Index, b, days, autoStake)
			if err != nil {
				return err
			}
			return c.apply(ctx, csm, legacyAuth)
		}
	}
	// Each case acts on bucket 0. Bucket 1 is candidate test1's self-stake
	// bucket unless the case says otherwise.
	selfStake1 := &bucketConfig{owner1, owner1, amount, 30, false, true, nil, 0}
	plain := func(dur uint32, auto bool, unstakeAt *time.Time, endorse uint64) *bucketConfig {
		return &bucketConfig{owner1, owner2, amount, dur, auto, false, unstakeAt, endorse}
	}
	for _, tc := range []struct {
		name    string
		bucket0 *bucketConfig
		blkTime time.Time
		run     run
		status  iotextypes.ReceiptStatus
	}{
		{"unstake", plain(30, false, nil, 0), timeBlock, unstake, iotextypes.ReceiptStatus_Success},
		{"unstake auto-stake", plain(30, true, nil, 0), timeBlock, unstake, iotextypes.ReceiptStatus_ErrInvalidBucketType},
		{"unstake before maturity", plain(30, false, nil, 0), early, unstake, iotextypes.ReceiptStatus_ErrUnstakeBeforeMaturity},
		{"unstake twice", plain(30, false, &timeBeforeBlockII, 0), timeBlock, unstake, iotextypes.ReceiptStatus_ErrInvalidBucketType},
		{"unstake endorsed", plain(30, false, nil, endorsementNotExpireHeight), timeBlock, unstake, iotextypes.ReceiptStatus_ErrInvalidBucketType},
		{"unstake self-stake", selfStake1, timeBlock, unstake, iotextypes.ReceiptStatus_ErrUnstakeBeforeMaturity},
		{"withdraw", plain(30, false, &timeBeforeBlockII, 0), timeBlock, withdraw, iotextypes.ReceiptStatus_Success},
		{"withdraw not unstaked", plain(30, false, nil, 0), timeBlock, withdraw, iotextypes.ReceiptStatus_ErrWithdrawBeforeUnstake},
		{"withdraw before maturity", plain(30, false, &recent, 0), timeBlock, withdraw, iotextypes.ReceiptStatus_ErrWithdrawBeforeMaturity},
		{"change candidate", plain(30, true, nil, 0), timeBlock, changeTo("test2"), iotextypes.ReceiptStatus_Success},
		{"change to same candidate", plain(30, true, nil, 0), timeBlock, changeTo("test1"), iotextypes.ReceiptStatus_ErrCandidateAlreadyExist},
		{"change candidate unstaked", plain(30, false, &timeBeforeBlockII, 0), timeBlock, changeTo("test2"), iotextypes.ReceiptStatus_ErrInvalidBucketType},
		{"change candidate of self-stake", selfStake1, timeBlock, changeTo("test2"), iotextypes.ReceiptStatus_Success},
		{"transfer", plain(30, true, nil, 0), timeBlock, transferTo(identityset.Address(3)), iotextypes.ReceiptStatus_Success},
		{"transfer to self", plain(30, true, nil, 0), timeBlock, transferTo(owner2), iotextypes.ReceiptStatus_ErrInvalidBucketType},
		{"restake longer", plain(30, true, nil, 0), timeBlock, restake(60, false), iotextypes.ReceiptStatus_Success},
		{"restake shorter with auto-stake", plain(30, true, nil, 0), timeBlock, restake(7, true), iotextypes.ReceiptStatus_ErrInvalidBucketType},
		{"restake shorter before maturity", plain(30, false, nil, 0), early, restake(7, false), iotextypes.ReceiptStatus_ErrReduceDurationBeforeMaturity},
		{"restake unstaked", plain(30, false, &timeBeforeBlockII, 0), timeBlock, restake(60, false), iotextypes.ReceiptStatus_ErrInvalidBucketType},
		{"restake self-stake", selfStake1, timeBlock, restake(91, true), iotextypes.ReceiptStatus_Success},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			ctrl := gomock.NewController(t)
			g := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
			g.ToBeEnabledBlockHeight = 1
			testutil.NormalizeGenesisHeights(&g.Blockchain)
			ctx := genesis.WithGenesisContext(context.Background(), g)
			ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: 10, BlockTimeStamp: tc.blkTime})
			ctx = protocol.WithFeatureCtx(ctx)
			r.True(protocol.MustGetFeatureCtx(ctx).EnableBucketGuard)

			buckets := []*bucketConfig{tc.bucket0}
			if tc.bucket0 != selfStake1 {
				buckets = append(buckets, selfStake1)
			}
			prepare := func(sm protocol.StateManager) (CandidateStateManager, *VoteBucket) {
				csm, err := NewCandidateStateManagerWithContext(ctx, sm)
				r.NoError(err)
				for i := range buckets {
					b, err := csm.NativeBucket(uint64(i))
					r.NoError(err)
					r.NoError(csm.DebitBucketPool(b.StakedAmount, true))
				}
				b, err := csm.NativeBucket(0)
				r.NoError(err)
				return csm, b
			}

			smA, pA, _, _ := initTestStateWithHeight(t, ctrl, buckets, cands, 10)
			csmA, bA := prepare(smA)
			errA := tc.run(ctx, pA, csmA, bA, false)

			smB, pB, _, _ := initTestStateWithHeight(t, ctrl, buckets, cands, 10)
			counted := &writeCountingSM{StateManager: smB}
			csmB, bB := prepare(counted)
			before := dumpStakingState(t, csmB, smB)
			counted.writes = 0
			errB := tc.run(ctx, pB, csmB, bB, true)

			r.Equal(tc.status.String(), receiptStatusOf(errB))
			r.Equal(receiptStatusOf(errA), receiptStatusOf(errB))
			if errB == nil {
				r.Equal(dumpStakingState(t, csmA, smA), dumpStakingState(t, csmB, smB))
				return
			}
			r.Zero(counted.writes, "a failed check must not write")
			r.Equal(before, dumpStakingState(t, csmB, smB))
		})
	}
}
