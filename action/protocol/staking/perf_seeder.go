// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"sort"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
)

// TestOnlyPerfBenchSpec configures TestOnlySeedPerfBenchState. Reachable only
// from e2etest/iip59_perf_test.go via the TestOnlyGenesisStateSeeder hook —
// production must never construct one.
type TestOnlyPerfBenchSpec struct {
	// NumDelegates is the count of delegates to plant. Each gets an
	// Candidate + self-stake bucket.
	NumDelegates int
	// NumVoters is the count of distinct voter buckets to plant. Voters
	// are distributed round-robin across the delegates.
	NumVoters int
	// DelegateSelfStake is the self-stake amount for every planted
	// candidate. Must be at least the network's SelfStakingThreshold or
	// downstream vote-weight calculations behave oddly.
	DelegateSelfStake *big.Int
	// VoterStake is the staked amount for every planted voter bucket.
	VoterStake *big.Int
	// VoterStakedDurationDays is the duration (in days) for voter buckets.
	// A 30-day auto-stake bucket routes into the compound path in the
	// IIP-59 drain.
	VoterStakedDurationDays uint32
	// VoteWeightCalConsts controls per-bucket vote weight — pass the
	// active genesis's copy so weight math matches production.
	VoteWeightCalConsts genesis.VoteWeightCalConsts
	// BlockCommissionBasisPoints is the frozen block-side commission rate
	// planted into each candidate's CandidatePollSnapshot. Downstream
	// GrantBlockReward reads this to split the base reward at block time.
	// Left at zero, the harness would route the full block reward straight
	// into the voter pool; that's rarely what a bench wants.
	BlockCommissionBasisPoints uint64
	// EpochCommissionBasisPoints is the frozen epoch-side commission rate
	// planted into each candidate's CandidatePollSnapshot.
	EpochCommissionBasisPoints uint64
}

// TestOnlySeedPerfBenchState plants NumDelegates candidates with
// self-stake buckets, then plants NumVoters voter buckets distributed
// round-robin. Returns the addresses of the planted delegates so the caller
// can look them up during the drain.
//
// Uses csm.putBucketAndIndex / csm.Upsert directly so the caller need not
// route registrations through the action pool — this makes 27 020-voter
// mainnet-tier seeding practical in a single genesis-block transaction.
//
// Intended solely for the e2e perf bench (task #68). Do not call from
// production paths.
func TestOnlySeedPerfBenchState(
	ctx context.Context,
	csm CandidateStateManager,
	spec TestOnlyPerfBenchSpec,
) ([]address.Address, error) {
	if spec.NumDelegates <= 0 {
		return nil, errors.New("perf-bench: NumDelegates must be positive")
	}
	if spec.NumVoters < 0 {
		return nil, errors.New("perf-bench: NumVoters must be non-negative")
	}
	if spec.DelegateSelfStake == nil || spec.DelegateSelfStake.Sign() <= 0 {
		return nil, errors.New("perf-bench: DelegateSelfStake must be positive")
	}
	if spec.VoterStake == nil || spec.VoterStake.Sign() <= 0 {
		return nil, errors.New("perf-bench: VoterStake must be positive")
	}
	if spec.VoterStakedDurationDays == 0 {
		spec.VoterStakedDurationDays = 30
	}
	blkCtx := protocol.MustGetBlockCtx(ctx)
	ts := blkCtx.BlockTimeStamp

	delegates := make([]address.Address, spec.NumDelegates)
	perDelegateVoters := make([][]VoterWeight, spec.NumDelegates)
	for i := 0; i < spec.NumDelegates; i++ {
		delAddr := perfBenchAddress(uint64(i) + 1)
		selfBkt := NewVoteBucket(delAddr, delAddr, new(big.Int).Set(spec.DelegateSelfStake), _perfBenchStakeDuration, ts, true)
		selfIdx, err := csm.putBucketAndIndex(selfBkt)
		if err != nil {
			return nil, errors.Wrapf(err, "put self-stake bucket for delegate %d", i)
		}
		cand := &Candidate{
			Owner:              delAddr,
			Operator:           delAddr,
			Reward:             delAddr,
			Name:               fmt.Sprintf("bdel%03d", i),
			Votes:              CalculateVoteWeight(spec.VoteWeightCalConsts, selfBkt, true),
			SelfStakeBucketIdx: selfIdx,
			SelfStake:          new(big.Int).Set(spec.DelegateSelfStake),
		}
		if err := csm.Upsert(cand); err != nil {
			return nil, errors.Wrapf(err, "upsert delegate candidate %d", i)
		}
		if err := csm.DebitBucketPool(spec.DelegateSelfStake, true); err != nil {
			return nil, errors.Wrapf(err, "debit bucket pool for delegate %d", i)
		}
		delegates[i] = delAddr
	}

	for j := 0; j < spec.NumVoters; j++ {
		voter := perfBenchAddress(uint64(j) + perfBenchVoterSeedBase)
		delIdx := j % spec.NumDelegates
		delAddr := delegates[delIdx]
		bkt := NewVoteBucket(delAddr, voter, new(big.Int).Set(spec.VoterStake), spec.VoterStakedDurationDays, ts, true)
		if _, err := csm.putBucketAndIndex(bkt); err != nil {
			return nil, errors.Wrapf(err, "put voter bucket %d", j)
		}
		cand := csm.GetByOwner(delAddr)
		if cand == nil {
			return nil, errors.Errorf("perf-bench: delegate %s missing after upsert", delAddr.String())
		}
		w := CalculateVoteWeight(spec.VoteWeightCalConsts, bkt, false)
		cand.Votes = new(big.Int).Add(cand.Votes, w)
		if err := csm.Upsert(cand); err != nil {
			return nil, errors.Wrapf(err, "reupsert delegate for voter %d", j)
		}
		if err := csm.DebitBucketPool(spec.VoterStake, false); err != nil {
			return nil, errors.Wrapf(err, "debit bucket pool for voter %d", j)
		}
		perDelegateVoters[delIdx] = append(perDelegateVoters[delIdx], VoterWeight{
			Voter:  voter,
			Weight: new(big.Int).Set(w),
		})
	}

	// Plant a frozen CandidatePollSnapshot per delegate. Production writes
	// this from PutPollResult via freezeIIP59PollSnapshot; the perf harness
	// runs on the LifeLong poll protocol, which never emits PutPollResult,
	// so the freeze would otherwise never happen and downstream rewarding
	// would land in the Registered=false fallback on every delegate.
	sm := csm.SM()
	for i, delAddr := range delegates {
		entries := perDelegateVoters[i]
		sort.Slice(entries, func(a, b int) bool {
			return bytes.Compare(entries[a].Voter.Bytes(), entries[b].Voter.Bytes()) < 0
		})
		snap := &CandidatePollSnapshot{
			OnchainRewardEnabled:       true,
			BlockCommissionBasisPoints: spec.BlockCommissionBasisPoints,
			EpochCommissionBasisPoints: spec.EpochCommissionBasisPoints,
			Registered:                 true,
			Entries:                    entries,
		}
		if _, err := sm.PutState(
			snap.toBlob(),
			protocol.NamespaceOption(_stakingNameSpace),
			protocol.KeyOption(candidatePollSnapshotKey(delAddr)),
		); err != nil {
			return nil, errors.Wrapf(err, "plant poll snapshot for delegate %d", i)
		}
	}
	return delegates, nil
}

// perfBenchVoterSeedBase separates the delegate address space (1..numDelegates)
// from the voter address space so collisions cannot happen at any sensible
// tier. 1e6 is comfortably above the mainnet-tier voter count (27 020).
const perfBenchVoterSeedBase uint64 = 1_000_000

// _perfBenchStakeDuration is the stake duration in days given to every seeded
// bucket. The value only has to clear the self-stake minimum; the benchmark
// cares about voter counts, not about weight curves.
const _perfBenchStakeDuration uint32 = 91

// perfBenchAddress derives a deterministic 20-byte address from a seed. The
// low 8 bytes carry the seed; the high 12 bytes are zero. Distinct seeds
// always produce distinct addresses.
func perfBenchAddress(seed uint64) address.Address {
	var b [20]byte
	binary.BigEndian.PutUint64(b[12:], seed)
	addr, err := address.FromBytes(b[:])
	if err != nil {
		panic(err)
	}
	return addr
}

// TestOnlyPerfBenchDelegateAddress returns the deterministic address the
// perf-bench seeder plants for delegate index i (0-based). The e2e harness
// uses this to build a matching genesis.Delegates list so the LifeLong poll
// protocol pays rewards to the seeded delegates rather than to
// identityset addresses.
func TestOnlyPerfBenchDelegateAddress(i int) address.Address {
	return perfBenchAddress(uint64(i) + 1)
}

// TestOnlyPerfBenchVoterAddress returns the deterministic address the
// perf-bench seeder plants for voter index j (0-based). Exported so tests
// asserting on planted voter state can round-trip the address.
func TestOnlyPerfBenchVoterAddress(j int) address.Address {
	return perfBenchAddress(uint64(j) + perfBenchVoterSeedBase)
}
