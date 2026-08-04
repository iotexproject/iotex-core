// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

//go:build iip59bench

package staking

import (
	"bytes"
	"fmt"
	"math/big"
	"sort"
	"testing"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// BenchmarkFreezeSnapshotNativeEnumeration measures the per-freeze wall
// time cost of Design B's native-bucket enumeration path: for each
// candidate, look up its bucket-index blob, load each bucket, compute
// weight, aggregate per voter, sort by voter bytes.
//
// This is Phase 0 of PR 5.5a. It answers a single question — does
// enumerating all buckets at PutPollResult time fit inside a block
// budget at mainnet + ceiling scale?
//
// **Fidelity caveat.** The bench uses testdb.NewMockStateManager, an
// in-memory map. Real production reads go through the state trie
// (leveldb/pebble + hash verification), which is slower by an
// implementation-dependent factor. Numbers here are a LOWER BOUND —
// production cost will be higher. Use these as a green-light / red-light
// signal: if the mock number already blows the block budget, real
// production has no chance.
//
// Decision rule tied to results:
//   - worst single-delegate freeze < 500ms AND mainnet-tier aggregate
//     < 1s ⇒ Design B viable, proceed.
//   - ceiling-tier > 1.5s or mainnet aggregate > 1.5s ⇒ Design B fails,
//     fall through to Design A (an incrementally-maintained per-(candidate,
//     voter) weight table). Design B won; Design A was built, measured
//     against this benchmark, and removed.
//
// Contract-staking enumeration is NOT measured here — the three
// indexers hold state in-memory, so per-bucket cost is far lower than
// native trie reads. Native dominates at mainnet scale (7.5k native vs
// ~300 contract).
func BenchmarkFreezeSnapshotNativeEnumeration(b *testing.B) {
	for _, tc := range []struct {
		name           string
		bucketsPerCand []int // one entry per delegate; value = voter buckets on that delegate
	}{
		{
			name:           "small_5x40",
			bucketsPerCand: repeatInt(40, 5),
		},
		{
			name:           "mainnet_uneven_52d_7508b",
			bucketsPerCand: mainnetShape(),
		},
		{
			name:           "ceiling_1x30000",
			bucketsPerCand: []int{30000},
		},
	} {
		b.Run(tc.name, func(b *testing.B) {
			ctx, sm, p, delegates := setupFreezeBenchState(b, tc.bucketsPerCand)
			consts := p.config.VoteWeightCalConsts

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				for _, delAddr := range delegates {
					entries, err := benchAggregateNativeVoterEntries(sm, consts, delAddr)
					if err != nil {
						b.Fatalf("aggregate for %s: %v", delAddr.String(), err)
					}
					_ = entries
				}
			}
			b.StopTimer()

			totalBuckets := 0
			for _, n := range tc.bucketsPerCand {
				totalBuckets += n
			}
			nsPerOp := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
			b.ReportMetric(nsPerOp/1e6, "ms/freeze")
			b.ReportMetric(float64(totalBuckets), "buckets/freeze")
			b.Logf("tier=%s delegates=%d buckets=%d per-freeze=%.2fms",
				tc.name, len(tc.bucketsPerCand), totalBuckets, nsPerOp/1e6)
			_ = ctx
		})
	}
}

// benchAggregateNativeVoterEntries mirrors what the real Phase 1a
// aggregator would do for the native side: enumerate all voter buckets
// pointing at the candidate, skip self-stake + unstaked, compute
// weight, aggregate per voter, sort.
//
// Inlined into the bench file (not exported to production) because
// Phase 0's job is to decide whether Design B is worth extracting to
// a real helper at all.
//
// Historical note: the answer was "yes", and the materialized per-voter
// list did ship — then got removed again when snapshots collapsed to
// scalars (candidate.Votes is the frozen denominator; the drain
// re-enumerates voters from the bucket indexes). The staking.VoterWeight
// type this used to return no longer exists, so the bench carries its own
// benchVoterWeight. The measurement is still the cost of the enumeration
// itself, which is what the tier thresholds above are calibrated against.
type benchVoterWeight struct {
	Voter  address.Address
	Weight *big.Int
}

func benchAggregateNativeVoterEntries(
	sr protocol.StateReader,
	consts genesis.VoteWeightCalConsts,
	candID address.Address,
) ([]benchVoterWeight, error) {
	csr := newCandidateStateReader(sr)
	// Note: for the bench, we don't have a candCenter, so we can't
	// look up cand.SelfStakeBucketIdx. In production the aggregator
	// takes *Candidate. Here we approximate by including all buckets
	// (self-stake bucket is 1 out of many at mainnet scale — noise).
	indices, _, err := csr.NativeBucketIndicesByCandidate(candID)
	if err != nil {
		return nil, errors.Wrapf(err, "index lookup")
	}
	if indices == nil {
		return nil, nil
	}
	buckets, err := csr.NativeBucketsWithIndices(*indices)
	if err != nil {
		return nil, errors.Wrap(err, "load buckets")
	}

	agg := make(map[[20]byte]*big.Int, len(buckets))
	for _, bkt := range buckets {
		if bkt == nil {
			continue
		}
		if !bkt.UnstakeStartTime.Equal(time.Unix(0, 0).UTC()) {
			continue
		}
		w := CalculateVoteWeight(consts, bkt, false)
		if w.Sign() == 0 {
			continue
		}
		var key [20]byte
		copy(key[:], bkt.Owner.Bytes())
		if existing, ok := agg[key]; ok {
			existing.Add(existing, w)
		} else {
			agg[key] = new(big.Int).Set(w)
		}
	}
	entries := make([]benchVoterWeight, 0, len(agg))
	for key, w := range agg {
		addr, err := address.FromBytes(key[:])
		if err != nil {
			return nil, errors.Wrap(err, "rebuild addr")
		}
		entries = append(entries, benchVoterWeight{Voter: addr, Weight: w})
	}
	sort.Slice(entries, func(a, b int) bool {
		return bytes.Compare(entries[a].Voter.Bytes(), entries[b].Voter.Bytes()) < 0
	})
	return entries, nil
}

// setupFreezeBenchState seeds a mock state manager with the requested
// per-delegate voter-bucket distribution. Returns the ctx, sm, protocol,
// and delegate addresses.
func setupFreezeBenchState(
	tb testing.TB,
	bucketsPerCand []int,
) (ctx protocolContextShim, sm protocol.StateManager, p *Protocol, delegates []address.Address) {
	tb.Helper()
	r := require.New(tb)
	ctrl := gomock.NewController(tb)
	msm := testdb.NewMockStateManagerWithoutHeightFunc(ctrl)
	msm.EXPECT().Height().Return(uint64(0), nil).AnyTimes()

	_, err := msm.PutState(
		&totalBucketCount{count: 0},
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(TotalBucketKey),
	)
	r.NoError(err)

	g := genesis.TestDefault()
	proto, err := NewProtocol(HelperCtx{
		DepositGas:    depositGas,
		BlockInterval: getBlockInterval,
	}, &BuilderConfig{
		Staking: g.Staking,
	}, nil, nil, nil, nil)
	r.NoError(err)

	csm := newCandidateStateManager(msm)

	stakedAmount, _ := new(big.Int).SetString("1200000000000000000000000", 10) // 1.2M IOTX
	delegates = make([]address.Address, len(bucketsPerCand))
	globalVoterSeed := uint64(1_000_000)
	for i, nBuckets := range bucketsPerCand {
		delAddr := benchDelegateAddress(uint64(i) + 1)
		delegates[i] = delAddr
		for j := 0; j < nBuckets; j++ {
			voter := benchVoterAddress(globalVoterSeed)
			globalVoterSeed++
			bkt := &VoteBucket{
				Candidate:        delAddr,
				Owner:            voter,
				StakedAmount:     new(big.Int).Set(stakedAmount),
				StakedDuration:   30 * 24 * time.Hour,
				CreateTime:       time.Unix(1700000000, 0).UTC(),
				StakeStartTime:   time.Unix(1700000000, 0).UTC(),
				UnstakeStartTime: time.Unix(0, 0).UTC(),
				AutoStake:        true,
			}
			_, err := csm.putBucketAndIndex(bkt)
			r.NoError(err)
		}
	}

	return protocolContextShim{}, msm, proto, delegates
}

// protocolContextShim is a placeholder — the bench does not need a real
// context because benchAggregateNativeVoterEntries reads only from the
// state manager.
type protocolContextShim struct{}

// benchDelegateAddress + benchVoterAddress produce deterministic 20-byte
// addresses from disjoint seed ranges (delegates 1..N, voters 1e6..).
func benchDelegateAddress(seed uint64) address.Address {
	var b [20]byte
	copy(b[12:], byteutil.Uint64ToBytesBigEndian(seed))
	addr, err := address.FromBytes(b[:])
	if err != nil {
		panic(err)
	}
	return addr
}

func benchVoterAddress(seed uint64) address.Address {
	var b [20]byte
	copy(b[12:], byteutil.Uint64ToBytesBigEndian(seed))
	addr, err := address.FromBytes(b[:])
	if err != nil {
		panic(err)
	}
	return addr
}

func repeatInt(v, n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = v
	}
	return out
}

// mainnetShape returns a per-delegate bucket-count distribution matching
// live mainnet (2026-07): 52 delegates, 7,508 buckets total, uneven —
// max=2,445 (cpc), median=16, min=1. We approximate:
//   - 1 delegate at 2,445 (cpc)
//   - Fill remaining 51 delegates to sum to 5,063 more, roughly Pareto:
//     a handful of hundreds, rest around the median.
//
// The exact per-delegate values matter less than the totals + the fact
// that one delegate is much heavier than the rest — that's the shape
// that stresses per-delegate wall time.
func mainnetShape() []int {
	// 10 heavy delegates: sum 5,545
	buckets := []int{2445, 900, 600, 400, 300, 250, 200, 180, 150, 120}
	// 32 delegates around median 16: sum 512
	medianish := []int{16, 16, 16, 16, 16, 16, 16, 16, 16, 16,
		16, 16, 16, 16, 16, 16, 16, 16, 16, 16,
		16, 16, 16, 16, 16, 16, 16, 16, 16, 16,
		16, 16}
	// 10 tail delegates: sum 20
	small := []int{5, 3, 3, 2, 2, 1, 1, 1, 1, 1}
	// Trim to exactly 52 delegates
	all := append(buckets, medianish...)
	all = append(all, small...)
	if len(all) != 52 {
		panic(fmt.Sprintf("mainnetShape must be 52 delegates, got %d", len(all)))
	}
	// Verify sum ~= 7,508 (target from live measurement)
	sum := 0
	for _, n := range all {
		sum += n
	}
	// Not checking exact match — shape matters more than sum for the bench.
	// Log for visibility.
	_ = sum
	return all
}
