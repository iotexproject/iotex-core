// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
)

// TestOnlySeedNativeVoterBucket plants one native vote bucket, bumps the total
// bucket count, and adds the index to the voter's and the candidate's bucket
// index lists. It returns the new bucket's index.
//
// It exists so tests outside this package -- the rewarding protocol's drain
// tests in particular -- can build the voter key space the IIP-59 address walk
// enumerates. bucketKey, AddrKeyWithPrefix and the _voterIndex / _candIndex
// tags are package-private, and writing those keys by hand from another package
// would encode this package's key layout in a test, which is exactly the
// coupling that makes a layout change unreviewable.
//
// It deliberately does not go through candSM. NewCandidateStateManager needs a
// live staking view, which the rewarding protocol's unit fixtures do not have,
// and going through candSM would also fire the era copy-on-write hooks -- the
// intended use is to plant state *before* a window is opened, so the drain sees
// it as pre-existing rather than as a post-freeze write.
//
// Test-only. Production bucket creation goes through the staking handlers,
// which also maintain the bucket pool and the candidate's vote accumulator;
// this does neither.
func TestOnlySeedNativeVoterBucket(
	sm protocol.StateManager,
	candidate, voter address.Address,
	amount *big.Int,
	durationDays uint32,
	ctime time.Time,
	autoStake bool,
) (uint64, error) {
	bucket := NewVoteBucket(candidate, voter, amount, durationDays, ctime, autoStake)
	var tc totalBucketCount
	if _, err := sm.State(
		&tc,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(TotalBucketKey),
	); err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return 0, err
	}
	index := tc.Count()
	bucket.Index = index
	if _, err := sm.PutState(bucket, nativeBucketStateOpts(index)...); err != nil {
		return 0, err
	}
	tc.count = index + 1
	if _, err := sm.PutState(
		&tc,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(TotalBucketKey),
	); err != nil {
		return 0, err
	}
	for _, entry := range []struct {
		addr   address.Address
		prefix byte
	}{{voter, _voterIndex}, {candidate, _candIndex}} {
		opts := nativeBucketIndexStateOpts(entry.addr, entry.prefix)
		var bis BucketIndices
		if _, err := sm.State(&bis, opts...); err != nil && errors.Cause(err) != state.ErrStateNotExist {
			return 0, err
		}
		bis.addBucketIndex(index)
		if _, err := sm.PutState(&bis, opts...); err != nil {
			return 0, err
		}
	}
	return index, nil
}

// TestOnlyPutVoterBucketThroughCOW plants one native vote bucket through a
// candidate state manager, so the IIP-59 copy-on-write hooks fire exactly as
// they do for a real staking action.
//
// This is the deliberate opposite of TestOnlySeedNativeVoterBucket above, which
// bypasses csm so that state it plants looks pre-existing. Use this one to
// model a bucket created *after* an era froze: the voter's index key gets a
// tombstone, so the era's view of that voter stays empty and the drain owes
// them nothing.
//
// Test-only. It maintains neither the candidate's vote accumulator nor the
// bucket pool, so nothing may assert on either afterwards.
func TestOnlyPutVoterBucketThroughCOW(
	ctx context.Context,
	sm protocol.StateManager,
	candidate, voter address.Address,
	amount *big.Int,
	durationDays uint32,
	ctime time.Time,
	autoStake bool,
) (uint64, error) {
	csm, err := NewCandidateStateManagerWithContext(ctx, sm)
	if err != nil {
		return 0, err
	}
	return csm.putBucketAndIndex(NewVoteBucket(candidate, voter, amount, durationDays, ctime, autoStake))
}

// TestOnlyDeleteVoterBucketsThroughCOW deletes every live native bucket a voter
// owns, through a candidate state manager so the copy-on-write hooks fire. It
// returns how many it deleted.
//
// This models the case the copy-on-write layer exists for: a voter who
// withdraws their last bucket mid-drain. Afterwards the voter has no live
// _voterIndex key at all, so only the era's copies can still name them — and
// they are still owed the share the era froze.
//
// Test-only, with the same caveats as TestOnlyPutVoterBucketThroughCOW: the
// candidate's Votes and the bucket pool are left untouched.
func TestOnlyDeleteVoterBucketsThroughCOW(
	ctx context.Context,
	sm protocol.StateManager,
	voter address.Address,
) (int, error) {
	csm, err := NewCandidateStateManagerWithContext(ctx, sm)
	if err != nil {
		return 0, err
	}
	var bis BucketIndices
	if _, err := sm.State(&bis, nativeBucketIndexStateOpts(voter, _voterIndex)...); err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			return 0, nil
		}
		return 0, err
	}
	// delBucketAndIndex rewrites the very list being ranged over, so take a copy
	// of the indices first.
	indices := append([]uint64(nil), bis...)
	for _, index := range indices {
		bkt, err := csm.NativeBucket(index)
		if err != nil {
			if errors.Cause(err) == state.ErrStateNotExist {
				continue
			}
			return 0, err
		}
		if err := csm.delBucketAndIndex(bkt.Owner, bkt.Candidate, index); err != nil {
			return 0, err
		}
	}
	return len(indices), nil
}

// TestOnlyPerfBenchSpec configures TestOnlySeedPerfBenchState. Reachable only
// from e2etest's IIP-59 benches, which pass the seeder to a single server via
// WithGenesisStateSeeder — production must never construct one.
type TestOnlyPerfBenchSpec struct {
	// NumDelegates is the count of delegates to plant. Each gets an
	// Candidate + self-stake bucket.
	NumDelegates int
	// NumVoters is the count of distinct voter buckets to plant. Voters
	// are distributed round-robin across the delegates.
	NumVoters int
	// NumNativeBuckets is the number of native voter buckets to plant across
	// NumVoters distinct owners. Zero preserves the original one-per-voter
	// fixture.
	NumNativeBuckets int
	// NumContractBuckets is the number of pre-activation contract-staking
	// buckets to plant. They are spread across ContractStakingAddresses and
	// deliberately written without a feature context, leaving owner indexes
	// absent for the activation backfill to build.
	NumContractBuckets int
	// ContractStakingAddresses are the contracts used for contract buckets.
	// At least one is required when NumContractBuckets is non-zero.
	ContractStakingAddresses []address.Address
	// SpreadVoterAddresses spreads voters across the full address key space.
	SpreadVoterAddresses bool
	// DeferContractBucketSeeding leaves contract bucket state for a harness to
	// plant after genesis while still including its weight in candidate totals.
	// This is needed by the real state factory, which filters contract-staking
	// namespaces from genesis before Xingu.
	DeferContractBucketSeeding bool
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
	// planted into each candidate's CandidateRewardSnapshot. Downstream
	// GrantBlockReward reads this to split the base reward at block time.
	// Left at zero, the harness would route the full block reward straight
	// into the voter pool; that's rarely what a bench wants.
	BlockCommissionBasisPoints uint64
	// EpochCommissionBasisPoints is the frozen epoch-side commission rate
	// planted into each candidate's CandidateRewardSnapshot.
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
	if spec.NumNativeBuckets < 0 || spec.NumContractBuckets < 0 {
		return nil, errors.New("perf-bench: bucket counts must be non-negative")
	}
	if spec.NumNativeBuckets == 0 {
		spec.NumNativeBuckets = spec.NumVoters
	}
	if spec.NumVoters == 0 && (spec.NumNativeBuckets > 0 || spec.NumContractBuckets > 0) {
		return nil, errors.New("perf-bench: positive bucket count requires NumVoters")
	}
	if spec.NumContractBuckets > 0 && len(spec.ContractStakingAddresses) == 0 {
		return nil, errors.New("perf-bench: contract buckets require ContractStakingAddresses")
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
	// The self-stake index is planted into each delegate's snapshot below. The
	// drain recomputes voter weights from the era's frozen buckets and decides
	// the self-stake bonus by comparing bucket index against this value, so a
	// snapshot that left it at zero would hand delegate 0's bonus to whoever
	// happens to own bucket 0.
	selfStakeIdx := make([]uint64, spec.NumDelegates)
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
		selfStakeIdx[i] = selfIdx
	}

	voterAddress := TestOnlyPerfBenchVoterAddress
	if spec.SpreadVoterAddresses {
		voterAddress = TestOnlyPerfBenchSpreadVoterAddress
	}
	voterWeight := CalculateVoteWeight(spec.VoteWeightCalConsts, NewVoteBucket(
		delegates[0], voterAddress(0), new(big.Int).Set(spec.VoterStake),
		spec.VoterStakedDurationDays, ts, true,
	), false)
	weightsByDelegate := make([]*big.Int, spec.NumDelegates)
	for i := range weightsByDelegate {
		weightsByDelegate[i] = new(big.Int)
	}

	for j := 0; j < spec.NumNativeBuckets; j++ {
		voterIdx := j % spec.NumVoters
		voter := voterAddress(voterIdx)
		delIdx := voterIdx % spec.NumDelegates
		delAddr := delegates[delIdx]
		bkt := NewVoteBucket(delAddr, voter, new(big.Int).Set(spec.VoterStake), spec.VoterStakedDurationDays, ts, true)
		if _, err := csm.putBucketAndIndex(bkt); err != nil {
			return nil, errors.Wrapf(err, "put native voter bucket %d", j)
		}
		weightsByDelegate[delIdx].Add(weightsByDelegate[delIdx], voterWeight)
		if err := csm.DebitBucketPool(spec.VoterStake, false); err != nil {
			return nil, errors.Wrapf(err, "debit bucket pool for voter %d", j)
		}
	}

	for j := 0; j < spec.NumContractBuckets; j++ {
		voterIdx := j % spec.NumVoters
		delIdx := voterIdx % spec.NumDelegates
		weightsByDelegate[delIdx].Add(weightsByDelegate[delIdx], voterWeight)
	}
	if !spec.DeferContractBucketSeeding {
		if err := TestOnlySeedPerfBenchContractBuckets(ctx, csm.SM(), spec); err != nil {
			return nil, err
		}
	}

	for i, delAddr := range delegates {
		cand := csm.GetByOwner(delAddr)
		if cand == nil {
			return nil, errors.Errorf("perf-bench: delegate %s missing after bucket seeding", delAddr.String())
		}
		cand.Votes = new(big.Int).Add(cand.Votes, weightsByDelegate[i])
		if err := csm.Upsert(cand); err != nil {
			return nil, errors.Wrapf(err, "reupsert delegate %d after voter seeding", i)
		}
	}

	// Plant a frozen CandidateRewardSnapshot per delegate. Production writes
	// this from PutPollResult via freezeIIP59RewardState; the perf harness
	// runs on the LifeLong poll protocol, which never emits PutPollResult,
	// so the freeze would otherwise never happen and downstream rewarding
	// would land in the CommissionConfigured=false fallback on every delegate.
	sm := csm.SM()
	for i, delAddr := range delegates {
		// TotalWeight is read back from the candidate exactly as the real
		// freezer reads it: the loop above kept cand.Votes in step with every
		// bucket it planted, so this is the seeded voter weight plus the
		// delegate's own self-stake weight -- and the self-stake bucket is a
		// voter the drain will visit.
		cand := csm.GetByOwner(delAddr)
		if cand == nil {
			return nil, errors.Errorf("perf-bench: delegate %s missing before snapshot", delAddr.String())
		}
		snap := &CandidateRewardSnapshot{
			BlockCommissionBasisPoints: spec.BlockCommissionBasisPoints,
			EpochCommissionBasisPoints: spec.EpochCommissionBasisPoints,
			CommissionConfigured:       true,
			SelfStakeBucketIdx:         selfStakeIdx[i],
			TotalWeight:                new(big.Int).Set(cand.Votes),
		}
		if _, err := sm.PutState(
			snap,
			protocol.NamespaceOption(_stakingNameSpace),
			protocol.KeyOption(candidateRewardSnapshotKey(delAddr)),
		); err != nil {
			return nil, errors.Wrapf(err, "plant reward snapshot for delegate %d", i)
		}
	}
	return delegates, nil
}

// TestOnlySeedPerfBenchContractBuckets plants the contract-bucket portion of a
// perf fixture. Callers that defer it run this in a pre-activation block after
// Xingu, so the real state factory persists the bucket namespaces while the
// IIP-59 owner-index gate is still shut.
func TestOnlySeedPerfBenchContractBuckets(
	ctx context.Context,
	sm protocol.StateManager,
	spec TestOnlyPerfBenchSpec,
) error {
	if spec.NumContractBuckets == 0 {
		return nil
	}
	if spec.NumDelegates <= 0 || spec.NumVoters <= 0 || len(spec.ContractStakingAddresses) == 0 ||
		spec.VoterStake == nil || spec.VoterStake.Sign() <= 0 {
		return errors.New("perf-bench: invalid deferred contract bucket fixture")
	}
	if spec.VoterStakedDurationDays == 0 {
		spec.VoterStakedDurationDays = 30
	}
	voterAddress := TestOnlyPerfBenchVoterAddress
	if spec.SpreadVoterAddresses {
		voterAddress = TestOnlyPerfBenchSpreadVoterAddress
	}
	ts := protocol.MustGetBlockCtx(ctx).BlockTimeStamp
	contractSM := contractstaking.NewContractStakingStateManager(sm)
	nextContractID := make([]uint64, len(spec.ContractStakingAddresses))
	for j := 0; j < spec.NumContractBuckets; j++ {
		voterIdx := j % spec.NumVoters
		contractIdx := j % len(spec.ContractStakingAddresses)
		id := nextContractID[contractIdx]
		nextContractID[contractIdx]++
		bucket := &contractstaking.Bucket{
			Candidate:        TestOnlyPerfBenchDelegateAddress(voterIdx % spec.NumDelegates),
			Owner:            voterAddress(voterIdx),
			StakedAmount:     new(big.Int).Set(spec.VoterStake),
			StakedDuration:   uint64(spec.VoterStakedDurationDays) * uint64(24*time.Hour/time.Second),
			CreatedAt:        uint64(ts.Unix()),
			UnlockedAt:       MaxDurationNumber,
			UnstakedAt:       MaxDurationNumber,
			IsTimestampBased: true,
		}
		if err := contractSM.UpsertBucket(ctx, spec.ContractStakingAddresses[contractIdx], id, bucket); err != nil {
			return errors.Wrapf(err, "put contract voter bucket %d", j)
		}
	}
	return nil
}

// perfBenchVoterSeedBase separates the delegate address space (1..numDelegates)
// from the voter address space so collisions cannot happen at any sensible
// tier. 1e6 is comfortably above the mainnet-tier voter count (27 020).
const perfBenchVoterSeedBase uint64 = 1_000_000

// _perfBenchStakeDuration is the stake duration in days given to every seeded
// bucket. The value only has to clear the self-stake minimum; the benchmark
// cares about voter counts, not about weight curves.
const _perfBenchStakeDuration uint32 = 91

// TestOnlyPerfBenchDelegateStakeDurationDays is the duration above, exported so
// a harness can reproduce a seeded delegate's self-stake weight from
// CalculateVoteWeight instead of reading it back out of the state the code
// under test also wrote.
const TestOnlyPerfBenchDelegateStakeDurationDays = _perfBenchStakeDuration

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

// TestOnlyPerfBenchSpreadVoterAddress returns a deterministic voter address
// whose first byte is j mod 256, spreading scale fixtures across the full
// ordered voter key space.
func TestOnlyPerfBenchSpreadVoterAddress(j int) address.Address {
	var b [20]byte
	b[0] = byte(j)
	binary.BigEndian.PutUint64(b[12:], uint64(j)+perfBenchVoterSeedBase)
	addr, err := address.FromBytes(b[:])
	if err != nil {
		panic(err)
	}
	return addr
}
