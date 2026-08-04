// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"bytes"
	"context"
	"encoding/binary"
	"math/big"
	"sort"
	"testing"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/eracow"
)

// This file builds the state a voter-major IIP-59 drain actually walks.
//
// The drain no longer reads CandidatePollSnapshot.Entries; it enumerates the
// voter key space and recomputes each voter's weight from the era's frozen
// buckets. A fixture that only writes poll snapshots -- which is what every
// rewarding test did before P5 -- therefore presents the drain with zero voters,
// and every assertion about who got paid becomes vacuous. So these helpers plant
// real buckets, real voter-index keys, and a real era copy-on-write window.
//
// The frozen TotalWeight is computed by calling the same FrozenVoterWeight the
// drain calls, rather than by writing a weight the test made up. That is
// deliberate: a hand-written total that disagreed with the recompute would put
// every fixture permanently inside the payout clamp, and the clamp test could
// then pass for the wrong reason.

// iip59NativeSeed is one native vote bucket to plant.
type iip59NativeSeed struct {
	delegate address.Address
	voter    address.Address
	amount   int64
	// durationDays defaults to 30 when zero.
	durationDays uint32
}

// iip59ContractSeed is one contract-staking (liquid staking) bucket to plant,
// together with its owner-index entry.
type iip59ContractSeed struct {
	delegate address.Address
	voter    address.Address
	amount   int64
	contract address.Address
	bucketID uint64
	// timestamped selects the bucket's clock. A false value is the case the
	// evalHeight regression test cares about: remaining duration is then
	// measured in blocks against the evaluation height, so the weight moves if
	// the drain leaks the current block height into the recompute.
	timestamped bool
	// duration is seconds when timestamped, blocks otherwise.
	duration uint64
	// createdAt is a unix timestamp when timestamped, a block height otherwise.
	createdAt uint64
}

// iip59DrainFixture is the planted state plus the weights the drain will
// recompute from it.
type iip59DrainFixture struct {
	freezeHeight uint64
	delegates    []address.Address
	voters       []address.Address
	// weight is keyed by delegate bytes then voter bytes.
	weight map[string]map[string]*big.Int
	// total is the per-delegate sum of weight, i.e. what Phase A freezes as
	// TotalWeight.
	total map[string]*big.Int
}

// iip59FixtureFreezeHeight is the era freeze height the fixtures open their
// window at. Any positive height works; a constant keeps the "evalHeight is the
// freeze height, not the block height" tests honest, because the block height
// the drain runs at is always something else.
const iip59FixtureFreezeHeight = uint64(64)

// seedIIP59DrainState plants buckets, opens the era window, computes the frozen
// weights, and writes one poll snapshot per delegate.
//
// Order matters. Everything is planted before the window opens so the drain sees
// it as state that existed at the freeze height; a bucket written after Begin
// would be rejected by the window's high-water marks, which is exactly what
// production wants and exactly what a fixture must avoid doing by accident.
func seedIIP59DrainState(
	t *testing.T,
	ctx context.Context,
	sm protocol.StateManager,
	freezeHeight uint64,
	natives []iip59NativeSeed,
	contracts []iip59ContractSeed,
) *iip59DrainFixture {
	t.Helper()
	r := require.New(t)

	pairs := make([][2]address.Address, 0, len(natives)+len(contracts))
	for _, s := range natives {
		days := s.durationDays
		if days == 0 {
			days = 30
		}
		_, err := staking.TestOnlySeedNativeVoterBucket(
			sm, s.delegate, s.voter, big.NewInt(s.amount), days, time.Unix(0, 0).UTC(), false,
		)
		r.NoError(err)
		pairs = append(pairs, [2]address.Address{s.delegate, s.voter})
	}

	if len(contracts) > 0 {
		csm := contractstaking.NewContractStakingStateManager(sm)
		highWater := make(map[string]uint64, len(contracts))
		contractAddrs := make([]address.Address, 0, len(contracts))
		for _, s := range contracts {
			bkt := &contractstaking.Bucket{
				Candidate:        s.delegate,
				Owner:            s.voter,
				StakedAmount:     big.NewInt(s.amount),
				StakedDuration:   s.duration,
				CreatedAt:        s.createdAt,
				UnlockedAt:       staking.MaxDurationNumber,
				UnstakedAt:       staking.MaxDurationNumber,
				IsTimestampBased: s.timestamped,
			}
			r.NoError(csm.UpsertBucket(ctx, s.contract, s.bucketID, bkt))
			key := string(s.contract.Bytes())
			if _, ok := highWater[key]; !ok {
				contractAddrs = append(contractAddrs, s.contract)
			}
			if s.bucketID > highWater[key] {
				highWater[key] = s.bucketID
			}
			pairs = append(pairs, [2]address.Address{s.delegate, s.voter})
		}
		// NumOfBuckets is a max-seen id, inclusive, so the mark is the largest
		// id planted rather than one past it.
		for _, contract := range contractAddrs {
			r.NoError(csm.UpdateNumOfBuckets(contract, highWater[string(contract.Bytes())]))
		}
	}

	r.NoError(staking.TestOnlyBeginEraCOWWindow(ctx, sm, freezeHeight))
	window, err := staking.EraCOWWindow(sm)
	r.NoError(err)
	r.True(window.Open(), "fixture must leave an era window open for the drain")

	f := &iip59DrainFixture{
		freezeHeight: freezeHeight,
		weight:       make(map[string]map[string]*big.Int),
		total:        make(map[string]*big.Int),
	}
	stakingProto := staking.FindProtocol(protocol.MustGetRegistry(ctx))
	r.NotNil(stakingProto, "fixture needs a registered staking protocol to recompute weights")

	seen := make(map[string]bool, len(pairs))
	voterSeen := make(map[string]bool, len(pairs))
	for _, pair := range pairs {
		delegate, voter := pair[0], pair[1]
		dk, vk := string(delegate.Bytes()), string(voter.Bytes())
		if !voterSeen[vk] {
			voterSeen[vk] = true
			f.voters = append(f.voters, voter)
		}
		if _, ok := f.weight[dk]; !ok {
			f.weight[dk] = make(map[string]*big.Int)
			f.total[dk] = new(big.Int)
			f.delegates = append(f.delegates, delegate)
		}
		if seen[dk+"|"+vk] {
			// FrozenVoterWeight already sums every bucket this voter holds with
			// this delegate, so a second seed for the same pair must not be
			// counted twice.
			continue
		}
		seen[dk+"|"+vk] = true
		w, wErr := staking.FrozenVoterWeight(
			sm, window, stakingProto, delegate, voter,
			staking.NoSelfStakeBucketIndex, freezeHeight,
		)
		r.NoError(wErr)
		f.weight[dk][vk] = w
		f.total[dk] = new(big.Int).Add(f.total[dk], w)
	}
	sortAddrs(f.delegates)
	sortAddrs(f.voters)

	for _, delegate := range f.delegates {
		dk := string(delegate.Bytes())
		// The frozen denominator. Production takes it from candidate.Votes;
		// the fixture takes the equivalent sum over FrozenVoterWeight for
		// every seeded pair, which is the same number by the invariant
		// TestVoterWeightInvariant pins (candidate.Votes == sum of per-voter
		// frozen weights). Keeping the fixture on the sum rather than
		// planting a candidate record means the drain's per-voter recompute
		// is still checked against an independently derived total.
		totalWeight := new(big.Int).Set(f.total[dk])
		r.NoError(staking.TestOnlyPutPollSnapshotFor(sm, delegate, &staking.CandidatePollSnapshot{
			OnchainRewardEnabled: true,
			// Zero commission: the whole epoch reward becomes the voter pool,
			// so a Phase A run over this fixture produces a non-zero
			// VoterAmountFrozen for Phase B to actually distribute.
			BlockCommissionBasisPoints: 0,
			EpochCommissionBasisPoints: 0,
			Registered:                 true,
			TotalWeight:                totalWeight,
			FreezeHeight:               freezeHeight,
			SelfStakeBucketIdx:         staking.NoSelfStakeBucketIndex,
		}))
	}
	return f
}

// weightOf returns the frozen weight of one (delegate, voter) pair, or zero.
func (f *iip59DrainFixture) weightOf(delegate, voter address.Address) *big.Int {
	byVoter, ok := f.weight[string(delegate.Bytes())]
	if !ok {
		return new(big.Int)
	}
	w, ok := byVoter[string(voter.Bytes())]
	if !ok {
		return new(big.Int)
	}
	return new(big.Int).Set(w)
}

// totalWeightOf returns the frozen TotalWeight of one delegate.
func (f *iip59DrainFixture) totalWeightOf(delegate address.Address) *big.Int {
	if w, ok := f.total[string(delegate.Bytes())]; ok {
		return new(big.Int).Set(w)
	}
	return new(big.Int)
}

// expectedShare is floor(pool * weight / totalWeight): the unclamped share the
// drain owes one voter from one delegate's pool.
func (f *iip59DrainFixture) expectedShare(delegate, voter address.Address, pool *big.Int) *big.Int {
	total := f.totalWeightOf(delegate)
	if total.Sign() <= 0 {
		return new(big.Int)
	}
	share := new(big.Int).Mul(safeBig(pool), f.weightOf(delegate, voter))
	return share.Div(share, total)
}

// sameShardVoter returns an address whose first byte -- and therefore whose
// key-space shard -- is fixed by the caller, so a fixture can force several
// voters into one shard.
//
// This exists because identityset addresses are hashes: the first 35 of them
// land in 35 distinct shards, so a drain seeded from them finishes every shard
// it enters and can never be stopped part-way through one. Any test about
// mid-shard resume needs addresses it controls the shard of.
func sameShardVoter(shard byte, n int) address.Address {
	var b [20]byte
	b[0] = shard
	binary.BigEndian.PutUint64(b[12:], uint64(n)+1)
	addr, err := address.FromBytes(b[:])
	if err != nil {
		panic(err)
	}
	return addr
}

// sortAddrs orders addresses by their raw bytes, which is the order every
// key-space walk in the drain produces.
func sortAddrs(addrs []address.Address) {
	sort.Slice(addrs, func(i, j int) bool {
		return bytes.Compare(addrs[i].Bytes(), addrs[j].Bytes()) < 0
	})
}

// accountBalances reads the primary account balance of each address. Voter
// payouts land here (creditPrimaryAccount), not in the per-address unclaimed
// reward balance, so this is what a payout assertion has to read.
func accountBalances(
	t *testing.T,
	sm protocol.StateReader,
	addrs []address.Address,
) map[string]*big.Int {
	t.Helper()
	out := make(map[string]*big.Int, len(addrs))
	for _, a := range addrs {
		acct, err := accountutil.LoadAccount(sm, a)
		require.NoError(t, err)
		out[a.String()] = new(big.Int).Set(acct.Balance)
	}
	return out
}

// drainPhaseBToCompletion drives GrantVoterRewardChunk until the cursor reports
// completion and returns how many chunk calls that took. Phase A must already
// have run.
func drainPhaseBToCompletion(
	t *testing.T,
	ctx context.Context,
	sm protocol.StateManager,
	p *Protocol,
) int {
	t.Helper()
	r := require.New(t)
	chunks := 0
	for {
		cursor, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		if cursor == nil || cursor.Completed {
			return chunks
		}
		_, _, err = p.GrantVoterRewardChunk(ctx, sm)
		r.NoError(err)
		chunks++
		if chunks > 2000 {
			t.Fatal("drain loop exceeded 2000 chunks — cursor is not advancing")
		}
	}
}

// openEraWindowForTest opens an era window with no planted buckets. Used by the
// tests that only need the drain to be allowed to run.
func openEraWindowForTest(
	t *testing.T,
	ctx context.Context,
	sm protocol.StateManager,
	freezeHeight uint64,
) eracow.Window {
	t.Helper()
	r := require.New(t)
	r.NoError(staking.TestOnlyBeginEraCOWWindow(ctx, sm, freezeHeight))
	window, err := staking.EraCOWWindow(sm)
	r.NoError(err)
	r.True(window.Open())
	return window
}
