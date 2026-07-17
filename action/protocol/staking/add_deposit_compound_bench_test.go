// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

//go:build iip59bench

package staking

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/mohae/deepcopy"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// BenchmarkAddDepositForCompound measures per-voter cost of the write path
// invoked once per opted-in voter during the epoch drain. Together with the
// slot-lookup path (SlotBucketReader.LookupBucket ≈ 2.5μs/voter, benched in
// protocol_iip59_bench_test.go), this determines the total drain cost at
// mainnet scale (~27k distinct voters — see docs/iip-59-perf-report.md).
//
// Structure: seed nCand candidates and nVoters voter+bucket pairs, then
// call AddDepositForCompound in a round-robin. The mutated bucket state
// persists across iterations (StakedAmount grows), which mirrors real
// drain semantics — each epoch further deposits into the same bucket.
func BenchmarkAddDepositForCompound(b *testing.B) {
	for _, tc := range []struct {
		name    string
		nCand   int
		nVoters int
	}{
		{"cand=10_voters=100", 10, 100},
		{"cand=100_voters=1000", 100, 1000},
		{"cand=100_voters=10000", 100, 10000},
	} {
		b.Run(tc.name, func(b *testing.B) {
			ctx, sm, p, voters, bucketIDs := setupCompoundBench(b, tc.nCand, tc.nVoters)
			amount := big.NewInt(1_000_000_000_000_000) // 0.001 IOTX per compound
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				idx := i % len(voters)
				if err := p.AddDepositForCompound(ctx, sm, voters[idx], bucketIDs[idx], amount); err != nil {
					b.Fatalf("compound at i=%d: %v", i, err)
				}
			}
		})
	}
}

// setupCompoundBench builds a state manager pre-populated with nCand
// candidates and nVoters voter-owned buckets, each bucket eligible for
// compound deposit (Owner=voter, AutoStake=true, not unstaked). Returns
// the parallel voters/bucketIDs slices for round-robin iteration.
func setupCompoundBench(
	tb testing.TB,
	nCand, nVoters int,
) (context.Context, protocol.StateManager, *Protocol, []address.Address, []uint64) {
	tb.Helper()
	r := require.New(tb)
	ctrl := gomock.NewController(tb)
	sm := testdb.NewMockStateManagerWithoutHeightFunc(ctrl)
	sm.EXPECT().Height().Return(uint64(0), nil).AnyTimes()

	_, err := sm.PutState(
		&totalBucketCount{count: 0},
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(TotalBucketKey),
	)
	r.NoError(err)

	g := genesis.TestDefault()
	p, err := NewProtocol(HelperCtx{
		DepositGas:    depositGas,
		BlockInterval: getBlockInterval,
	}, &BuilderConfig{
		Staking:                       g.Staking,
		PersistStakingPatchBlock:      math.MaxUint64,
		SkipContractStakingViewHeight: math.MaxUint64,
		Revise: ReviseConfig{
			VoteWeight: g.Staking.VoteWeightCalConsts,
		},
	}, nil, nil, nil, nil)
	r.NoError(err)

	csm := newCandidateStateManager(sm)

	candidates := make([]address.Address, nCand)
	for i := 0; i < nCand; i++ {
		candidates[i] = genBenchAddress(uint64(i) + 1)
	}

	stakedAmount, _ := new(big.Int).SetString("1200000000000000000000000", 10) // 1.2M IOTX
	voters := make([]address.Address, nVoters)
	bucketIDs := make([]uint64, nVoters)
	candVotes := make(map[string]*big.Int, nCand)
	for i := 0; i < nVoters; i++ {
		voter := genBenchAddress(uint64(i) + 1_000_000)
		cand := candidates[i%nCand]
		bkt := &VoteBucket{
			Candidate:        cand,
			Owner:            voter,
			StakedAmount:     new(big.Int).Set(stakedAmount),
			StakedDuration:   30 * 24 * time.Hour,
			CreateTime:       timeBeforeBlockI,
			StakeStartTime:   timeBeforeBlockI,
			UnstakeStartTime: time.Unix(0, 0).UTC(),
			AutoStake:        true,
		}
		idx, err := csm.putBucketAndIndex(bkt)
		r.NoError(err)
		voters[i] = voter
		bucketIDs[i] = idx
		w := p.calculateVoteWeight(bkt, false)
		if _, ok := candVotes[cand.String()]; !ok {
			candVotes[cand.String()] = big.NewInt(0)
		}
		candVotes[cand.String()].Add(candVotes[cand.String()], w)
	}

	for i, candOwner := range candidates {
		votes := big.NewInt(0)
		if v, ok := candVotes[candOwner.String()]; ok {
			votes = v
		}
		cand := &Candidate{
			Owner:              candOwner,
			Operator:           candOwner,
			Reward:             candOwner,
			Name:               fmt.Sprintf("cand%08d", i),
			Votes:              votes,
			SelfStakeBucketIdx: uint64(candidateNoSelfStakeBucketIndex),
			SelfStake:          big.NewInt(0),
		}
		r.NoError(csm.putCandidate(cand))
	}

	cfg := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
	ctx := genesis.WithGenesisContext(context.Background(), cfg)
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	ctx = protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{}))
	v, err := p.Start(ctx, sm)
	r.NoError(err)
	vd, ok := v.(*viewData)
	r.True(ok)
	r.NoError(sm.WriteView(_protocolID, vd))

	return ctx, sm, p, voters, bucketIDs
}

// genBenchAddress synthesises a deterministic 20-byte address from a seed.
// Distinct seeds always produce distinct addresses; the range starts above
// 1e6 for voters and 1..nCand for candidates so the two spaces never
// collide inside a single bench setup.
func genBenchAddress(seed uint64) address.Address {
	var b [20]byte
	binary.BigEndian.PutUint64(b[12:], seed)
	addr, err := address.FromBytes(b[:])
	if err != nil {
		panic(err)
	}
	return addr
}
