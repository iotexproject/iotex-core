// Copyright (c) 2026 IoTeX Foundation
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"bytes"
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// One shared state manager, delegate, reward pool and distribution cursor:
// exercise production routing and fund booking for all three bucket classes.
func TestPayVoterCombinedVaultBatchConservation(t *testing.T) {
	f := newCompoundFixtureWithVoters(t, 3)
	g := genesis.MustExtractGenesisContext(f.ctx)
	g.ToBeEnabledBlockHeight = 1
	g.ZanzibarGammaBlockHeight = 1

	f.ctx = protocol.WithFeatureCtx(genesis.WithGenesisContext(f.ctx, g))
	buckets := make([]*staking.VoteBucket, 3)
	for i := range buckets {
		voter := staking.TestOnlyPerfBenchVoterAddress(i)
		all, _, err := f.routing.csr.NativeBuckets()
		require.NoError(t, err)
		for _, b := range all {
			if bytes.Equal(b.Owner.Bytes(), voter.Bytes()) {
				buckets[i] = b
				break
			}
		}
		require.NotNil(t, buckets[i])
	}
	guards := make([]*staking.BucketGuardV2Record, 3)
	for i := 1; i < 3; i++ {
		guards[i] = &staking.BucketGuardV2Record{BucketGuardV2: bucketguard.BucketGuardV2{
			BucketClass: bucketguard.ClassBStaking, AlgID: pqcrypto.AlgSLHDSA192s, PublicKey: bytes.Repeat([]byte{byte(i)}, 48), Generation: 2, GuardNonce: uint64(10 + i), NextRequestID: 2, BoundHeight: 1, UnbindDelay: bucketguard.UnbindDelayBlocks,
		}}
		if i == 2 {
			guards[i].Pending = &bucketguard.PendingUnbind{RequestID: 1, StartHeight: 90, ExpiresAt: 90 + bucketguard.UnbindDelayBlocks}
		}
		_, err := f.sm.PutState(guards[i], protocol.NamespaceOption(state.StakingNamespace), protocol.KeyOption(binary.BigEndian.AppendUint64([]byte{13}, buckets[i].Index)))
		require.NoError(t, err)
		// The deployed write firewall and V2 recovery record both remain unchanged.
		_, err = f.sm.PutState(&staking.BucketGuardRecord{Guard: bucketguard.Guard{AlgID: guards[i].AlgID, PublicKey: guards[i].PublicKey, Generation: guards[i].Generation, Nonce: guards[i].GuardNonce, BoundHeight: 1}}, protocol.NamespaceOption(state.StakingNamespace), protocol.KeyOption(binary.BigEndian.AppendUint64([]byte{9}, buckets[i].Index)))
		require.NoError(t, err)
	}
	destination := identityset.Address(28)
	_, _, err := f.p.setVoterRewardDestination(f.ctx, f.sm, buckets[2].Owner, destination.Bytes())
	require.NoError(t, err)
	amounts := []*big.Int{big.NewInt(111), big.NewInt(222), big.NewInt(333)}
	total := big.NewInt(666)
	initialFund := big.NewInt(10000)
	require.NoError(t, f.p.putState(f.ctx, f.sm, _fundKey, &fund{totalBalance: initialFund, unclaimedBalance: big.NewInt(9334)}))
	require.NoError(t, f.p.creditPendingBlockRewardPool(f.ctx, f.sm, f.delegate.Bytes(), total))
	_, in := newRoutingShares(f.delegate, total)
	cursor := &voterRewardDistributionState{}
	cursor.DelegateAllocations = in.delegates
	cursor.DistributedByDelegate = []*big.Int{new(big.Int)}
	stakedBefore := new(big.Int).Set(f.routing.csr.TotalStakedAmount())
	compoundTotal := new(big.Int)
	directTotal := new(big.Int)
	for i, b := range buckets {
		f.routing.bucketReader = &registeredBucketReader{voter: b.Owner, bucketID: b.Index}
		shares, _ := newRoutingShares(f.delegate, amounts[i])
		payout, err := f.p.payVoterCombined(f.ctx, f.sm, f.routing, in, b.Owner, shares, &iip59RouteDurations{})
		require.NoError(t, err)
		require.Equal(t, i != 2, payout.compounded)
		require.Equal(t, amounts[i], payout.amount)
		require.NoError(t, f.p.bookVoterPayout(f.ctx, f.sm, cursor, payout))
		txlog := voterTransactionLog(payout)
		if i == 2 {
			require.Equal(t, destination.String(), payout.recipient.String())
			require.NotNil(t, txlog)
			require.Equal(t, destination.String(), txlog.Recipient)
			directTotal.Add(directTotal, txlog.Amount)
		} else {
			compoundTotal.Add(compoundTotal, payout.amount)
		}
		updated, err := f.routing.csr.NativeBucket(b.Index)
		require.NoError(t, err)
		expected := new(big.Int).Set(b.StakedAmount)
		if i != 2 {
			expected.Add(expected, amounts[i])
		}
		require.Equal(t, expected, updated.StakedAmount)
		ownerAcct, err := accountutil.LoadAccount(f.sm, b.Owner)
		require.NoError(t, err)
		require.Zero(t, ownerAcct.Balance.Sign(), "no duplicate direct payment")
		if i != 0 {
			got, err := staking.ReadStakingGuardV2(f.sm, b.Index)
			require.NoError(t, err)
			require.Equal(t, guards[i], got, "compound and direct reward must not consume guardNonce")
			var legacy staking.BucketGuardRecord
			_, err = f.sm.State(&legacy, protocol.NamespaceOption(state.StakingNamespace), protocol.KeyOption(binary.BigEndian.AppendUint64([]byte{9}, b.Index)))
			require.NoError(t, err)
			require.Equal(t, guards[i].GuardNonce, legacy.Nonce)
		}
	}
	acct, err := accountutil.LoadAccount(f.sm, destination)
	require.NoError(t, err)
	require.Equal(t, amounts[2], acct.Balance)
	csm, err := staking.NewCandidateStateManagerWithContext(f.ctx, f.sm)
	require.NoError(t, err)
	require.NoError(t, csm.Commit(f.ctx))
	csr, err := staking.ConstructBaseView(f.sm)
	require.NoError(t, err)
	require.Equal(t, compoundTotal, new(big.Int).Sub(csr.TotalStakedAmount(), stakedBefore))
	require.Equal(t, total, new(big.Int).Add(compoundTotal, directTotal))
	var after fund
	_, err = f.p.state(f.ctx, f.sm, _fundKey, &after)
	require.NoError(t, err)
	require.Equal(t, total, new(big.Int).Sub(initialFund, after.totalBalance))
	require.Equal(t, after.totalBalance, after.unclaimedBalance)
	pending, err := f.p.readPendingBlockRewardPool(f.ctx, f.sm, f.delegate.Bytes())
	require.NoError(t, err)
	require.Zero(t, pending.Sign())
	require.Equal(t, total, cursor.DistributedByDelegate[0])
}

type failingRecoveryRead struct {
	protocol.StateManager
	err error
}

func (s failingRecoveryRead) State(v interface{}, opts ...protocol.StateOption) (uint64, error) {
	if _, ok := v.(*staking.BucketGuardV2Record); ok {
		return 0, s.err
	}
	return s.StateManager.State(v, opts...)
}

func TestPayVoterCombinedPropagatesRecoveryReadFailure(t *testing.T) {
	f := newCompoundFixture(t)
	g := genesis.MustExtractGenesisContext(f.ctx)
	g.ToBeEnabledBlockHeight = 1
	g.ZanzibarGammaBlockHeight = 1

	f.ctx = protocol.WithFeatureCtx(genesis.WithGenesisContext(f.ctx, g))
	fault := errors.New("recovery metadata backend failure")
	sm := failingRecoveryRead{StateManager: f.sm, err: fault}
	err := f.routing.stakingProto.AddDepositForCompound(f.ctx, sm, f.voter, f.bucket.Index, big.NewInt(77), staking.FrozenSelfStake{})
	require.ErrorIs(t, err, fault)
	require.NotErrorIs(t, err, staking.ErrCompoundSkippedPendingUnbind)
	shares, in := newRoutingShares(f.delegate, big.NewInt(77))
	_, err = f.p.payVoterCombined(f.ctx, sm, f.routing, in, f.voter, shares, &iip59RouteDurations{})
	require.ErrorIs(t, err, fault)
	acct, err := accountutil.LoadAccount(f.sm, f.voter)
	require.NoError(t, err)
	require.Zero(t, acct.Balance.Sign())
	updated, err := f.routing.csr.NativeBucket(f.bucket.Index)
	require.NoError(t, err)
	require.Equal(t, f.bucket.StakedAmount, updated.StakedAmount)
}
