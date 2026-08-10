// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// The era's poll snapshot is the sole authority on opt-in status. Two kinds of
// delegate can reach a payout with no snapshot at all: one registered after the
// freeze height H (unfreezable by construction), and one that existed at H
// outside the poll list and opted in afterwards. Both are the same real-world
// situation as a candidate INSIDE the poll list that opts in after H — and that
// one is frozen as an OnchainRewardEnabled=false placeholder, which routes it
// to the legacy path.
//
// Before the fix the two diverged on nothing but poll-list membership at H: the
// placeholder case paid legacy, the missing case stayed on the LIVE opt-in flag
// and so paid on IIP-59 rails at the 100% commission default — the whole amount
// to the owner, nothing to the voters, no error and no log. For a Hermes-vault
// delegate that also bypassed the vault, so off-chain Hermes never saw the
// money and its voters lost it permanently.
//
// newVoterRewardCtx builds exactly the delegate at issue: owner is
// identityset.Address(1), its reward address is identityset.Address(2), and
// that address is the configured Hermes vault and its persisted opt-in bit is
// set. Writing no snapshot is the whole setup.

// TestResolveDelegateRewardRouting_MissingSnapshotIsOffRails is the fix.
func TestResolveDelegateRewardRouting_MissingSnapshotIsOffRails(t *testing.T) {
	r := require.New(t)
	_, sm, _, _, candAddr := newVoterRewardCtx(t, true)

	// Precondition: the delegate really is opted in as of live state, so the
	// assertions below are about the snapshot and not about the opt-in test.
	live, _, err := staking.NewCandidateByAddressReader(sm).CandidateByAddress(candAddr)
	r.NoError(err)
	r.NotNil(live)
	r.True(live.VoterRewardOnchainOptIn,
		"harness must present a live-opted-in delegate or this test proves nothing")

	_, err = staking.PollSnapshotFor(sm, candAddr)
	r.ErrorIs(err, state.ErrStateNotExist, "no snapshot is the condition under test")

	routing, err := resolveDelegateRewardRouting(sm, candAddr)
	r.NoError(err)
	r.False(routing.onchainRewardEnabled,
		"no snapshot at H means this era is not on IIP-59 rails, whatever live state says")

	addr := routing.PayoutAddress()
	r.Equal(identityset.Address(2).String(), addr.String(),
		"payout must go to the legacy reward address (the Hermes vault), not the owner")
	r.NotEqual(identityset.Address(1).String(), addr.String())
}

// TestResolveDelegateRewardRouting_MissingAgreesWithPlaceholder is the point of
// the change: the two ways of expressing "not opted in for this era" must be
// indistinguishable at the payout. A fix that only special-cased the missing
// case without matching the placeholder would leave the asymmetry in place.
func TestResolveDelegateRewardRouting_MissingAgreesWithPlaceholder(t *testing.T) {
	r := require.New(t)

	resolve := func(withPlaceholder bool) (*delegateRewardRouting, address.Address) {
		_, sm, _, _, candAddr := newVoterRewardCtx(t, true)
		if withPlaceholder {
			// Exactly what FreezePollSnapshot writes for a candidate in the
			// poll list that was not opted in at H.
			r.NoError(staking.TestOnlyPutPollSnapshotFor(sm, candAddr, &staking.CandidatePollSnapshot{
				OnchainRewardEnabled: false,
				FreezeHeight:         20_000,
				SelfStakeBucketIdx:   staking.NoSelfStakeBucketIndex,
				TotalWeight:          new(big.Int),
			}))
		}
		routing, err := resolveDelegateRewardRouting(sm, candAddr)
		r.NoError(err)
		return routing, routing.PayoutAddress()
	}

	missing, missingAddr := resolve(false)
	placeholder, placeholderAddr := resolve(true)

	r.Equal(placeholder.onchainRewardEnabled, missing.onchainRewardEnabled)
	r.Equal(placeholder.blockCommissionBPs, missing.blockCommissionBPs)
	r.Equal(placeholder.epochCommissionBPs, missing.epochCommissionBPs)
	r.Equal(placeholderAddr.String(), missingAddr.String())
}

// TestDistributeEpochCommissions_MissingSnapshotPaysLegacy walks the real epoch
// caller. The routing decision is only half the fix; this pins the observable
// consequence, which is a different payment mechanism and a different
// destination:
//
//   - legacy  -> grantToAccount, visible as an unclaimed balance, no transaction log
//   - IIP-59  -> creditRewardDirect, an immediate transfer with a
//     CLAIM_FROM_REWARDING_FUND transaction log
//
// Note that splitDelegateEpochReward returns (amount, 0) either way, so it is
// not itself a change detector — the split is 100%-to-delegate under the old
// default and short-circuits under the new one. Only the destination tells them
// apart.
func TestDistributeEpochCommissions_MissingSnapshotPaysLegacy(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, cand, candAddr := newVoterRewardCtx(t, true)
	openEraWindowForTest(t, ctx, sm, 20_000)

	out := &epochGrantResult{
		transactionLogs: make([]*action.TransactionLog, 0),
		rewardLogs:      make([]*action.Log, 0),
		debit:           big.NewInt(0),
	}
	r.NoError(p.distributeEpochCommissions(ctx, sm, epochCommissionInputs{
		rewardedCandidates: []*state.Candidate{cand},
		// Production passes delegateRewardRouting.PayoutAddress's answer here;
		// the pre-fork declared address is only a fallback.
		addrs:   []address.Address{identityset.Address(2)},
		amounts: []*big.Int{big.NewInt(1_000)},
	}, out))

	vaultBalance, _, err := p.UnclaimedBalance(ctx, sm, identityset.Address(2))
	r.NoError(err)
	r.Zero(big.NewInt(1_000).Cmp(vaultBalance),
		"the whole amount must land in the vault's claimable balance, where off-chain Hermes finds it")

	ownerBalance, _, err := p.UnclaimedBalance(ctx, sm, identityset.Address(1))
	r.NoError(err)
	r.Zero(ownerBalance.Sign(), "the owner must not be paid on a legacy grant")

	r.Empty(out.transactionLogs,
		"grantToAccount emits no transaction log; a CLAIM_FROM_REWARDING_FUND here means the IIP-59 path ran")

	// No voter money was withheld, so there is no pool and nothing for the
	// drain to settle.
	pool, err := p.readPendingBlockRewardPool(ctx, sm, candAddr.Bytes())
	r.NoError(err)
	r.Zero(pool.Sign())
}

// TestCreditBlockProducer_MissingSnapshotPaysLegacy covers the other payment
// path, which had the identical hole. The payout is assembled here the way
// resolveBlockProducerPayout assembles it (reward.go, post-fork branch) so the
// routing-to-onchainPool seam is what is under test; calling that function
// directly would need a poll protocol in the registry to find the producer.
func TestCreditBlockProducer_MissingSnapshotPaysLegacy(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, candAddr := newVoterRewardCtx(t, true)

	routing, err := resolveDelegateRewardRouting(sm, candAddr)
	r.NoError(err)
	payout := blockProducerPayout{
		addr:        routing.PayoutAddress(),
		candAddr:    candAddr,
		onchainPool: routing.onchainRewardEnabled,
	}
	if payout.onchainPool {
		payout.commissionBPs = routing.blockCommissionBPs
	}
	r.False(payout.onchainPool)

	var (
		blockReward  = big.NewInt(800)
		effectiveTip = big.NewInt(200)
		totalReward  = big.NewInt(1_000)
	)
	named, tLogs, err := p.creditBlockProducer(ctx, sm, payout, totalReward, blockReward, effectiveTip)
	r.NoError(err)
	r.Zero(blockReward.Cmp(named), "the BLOCK_REWARD log names the block reward, tip excluded")
	r.Empty(tLogs)

	vaultBalance, _, err := p.UnclaimedBalance(ctx, sm, identityset.Address(2))
	r.NoError(err)
	r.Zero(totalReward.Cmp(vaultBalance),
		"pre-IIP-59 semantics: block reward and tip together become one claimable balance")

	pool, err := p.readPendingBlockRewardPool(ctx, sm, candAddr.Bytes())
	r.NoError(err)
	r.Zero(pool.Sign())
}

// TestDistributeEpochCommissions_FreshSnapshotStaysOnRails is the regression
// lock on the other side. An over-broad version of the fix — one that keyed on
// anything but "no snapshot record" — would push every delegate onto the legacy
// path and IIP-59 would pay no voters at all.
func TestDistributeEpochCommissions_FreshSnapshotStaysOnRails(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, cand, candAddr := newVoterRewardCtx(t, true)

	const currentEraH = uint64(20_000)
	openEraWindowForTest(t, ctx, sm, currentEraH)
	r.NoError(staking.TestOnlyPutPollSnapshotFor(sm, candAddr, &staking.CandidatePollSnapshot{
		OnchainRewardEnabled: true,
		// All to voters, so the commission leg never touches the fund and this
		// test stays about the routing decision.
		EpochCommissionBasisPoints: 0,
		BlockCommissionBasisPoints: 0,
		TotalWeight:                big.NewInt(1_000_000),
		FreezeHeight:               currentEraH,
		SelfStakeBucketIdx:         7,
	}))

	routing, err := resolveDelegateRewardRouting(sm, candAddr)
	r.NoError(err)
	r.True(routing.onchainRewardEnabled)

	out := &epochGrantResult{
		transactionLogs: make([]*action.TransactionLog, 0),
		rewardLogs:      make([]*action.Log, 0),
		debit:           big.NewInt(0),
	}
	r.NoError(p.distributeEpochCommissions(ctx, sm, epochCommissionInputs{
		rewardedCandidates: []*state.Candidate{cand},
		addrs:              []address.Address{identityset.Address(2)},
		amounts:            []*big.Int{big.NewInt(1_000)},
	}, out))

	pool, err := p.readPendingBlockRewardPool(ctx, sm, candAddr.Bytes())
	r.NoError(err)
	r.Zero(big.NewInt(1_000).Cmp(pool), "voter money must still accrue into the pending pool")

	work, err := p.freezePendingPoolDrainWork(ctx, sm, currentEraH)
	r.NoError(err)
	r.Len(work, 1, "a fresh positive-weight snapshot must enter the immutable drain plan")
	r.Equal(candAddr.Bytes(), work[0].CandidateIdentifier)
	r.Zero(big.NewInt(1_000).Cmp(work[0].VoterAmountFrozen))
	r.Zero(big.NewInt(1_000_000).Cmp(work[0].TotalWeight))
	r.Equal(uint64(7), work[0].SelfStakeBucketIdx)

	poolAfterPlan, err := p.readPendingBlockRewardPool(ctx, sm, candAddr.Bytes())
	r.NoError(err)
	r.Zero(pool.Cmp(poolAfterPlan), "building the plan must not consume the pending pool")

	vaultBalance, _, err := p.UnclaimedBalance(ctx, sm, identityset.Address(2))
	r.NoError(err)
	r.Zero(vaultBalance.Sign(), "an on-rails delegate must not fall back to a legacy grant")
}
