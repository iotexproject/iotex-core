package staking

import (
	"context"
	"testing"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestCandidatesBucketsIndexer_StartStop(t *testing.T) {
	require := require.New(t)

	ctx := context.Background()
	store := db.NewMemKVStore()
	cbi, err := NewStakingCandidatesBucketsIndexer(store)
	require.NoError(err)

	require.NoError(cbi.Start(ctx))
	require.NoError(cbi.Stop(ctx))
}

func TestCandidatesBucketsIndexer_PutGetCandidates(t *testing.T) {
	require := require.New(t)

	ctx := context.Background()
	store := db.NewMemKVStore()
	cbi, err := NewStakingCandidatesBucketsIndexer(store)
	require.NoError(err)

	require.NoError(cbi.Start(ctx))

	candidates := &iotextypes.CandidateListV2{}
	candidates.Candidates = append(candidates.Candidates, &iotextypes.CandidateV2{
		Name:               "abc",
		SelfStakeBucketIdx: 123,
	})

	require.NoError(cbi.PutCandidates(0, nil))

	tests := []struct {
		height     uint64
		candidates *iotextypes.CandidateListV2
	}{
		{1, &iotextypes.CandidateListV2{}},
		{2, candidates},
	}

	for _, v := range tests {
		require.NoError(cbi.PutCandidates(v.height, v.candidates))
	}

	tests2 := []struct {
		height        uint64
		offset, limit uint32
		r1            int
		r2            uint64
	}{
		{0, 0, 0, 0, 0},
		{2, 0, 1, 1, 2},
		{2, 0, 100, 1, 2},
		{2, 1234, 0, 0, 2},
		{2, 1111111, 1, 0, 2},
		{21111111, 1111111, 1333333, 0, 2},
	}
	for _, v2 := range tests2 {
		a, b, err := cbi.GetCandidates(v2.height, v2.offset, v2.limit)
		require.NoError(err)

		var r iotextypes.CandidateListV2
		require.NoError(proto.Unmarshal(a, &r))
		require.Equal(len(r.Candidates), v2.r1)
		require.Equal(b, v2.r2)

		if len(r.Candidates) > 0 {
			require.Equal(r.Candidates[0].Name, tests[1].candidates.Candidates[0].Name)
			require.Equal(r.Candidates[0].SelfStakeBucketIdx, tests[1].candidates.Candidates[0].SelfStakeBucketIdx)
		}
	}
	require.NoError(cbi.Stop(ctx))
}

func TestCandidatesBucketsIndexer_PutGetBuckets(t *testing.T) {
	require := require.New(t)

	ctx := context.Background()
	store := db.NewMemKVStore()
	cbi, err := NewStakingCandidatesBucketsIndexer(store)
	require.NoError(err)

	require.NoError(cbi.Start(ctx))

	voteBuckets := &iotextypes.VoteBucketList{}
	voteBuckets.Buckets = append(voteBuckets.Buckets, &iotextypes.VoteBucket{
		Index:     uint64(1234),
		AutoStake: true,
	})

	require.NoError(cbi.PutBuckets(0, nil))

	tests := []struct {
		height  uint64
		buckets *iotextypes.VoteBucketList
	}{
		{1, &iotextypes.VoteBucketList{}},
		{2, voteBuckets},
	}

	for _, v := range tests {
		require.NoError(cbi.PutBuckets(v.height, v.buckets))
	}

	tests2 := []struct {
		height        uint64
		offset, limit uint32
		r1            int
		r2            uint64
	}{
		{0, 0, 0, 0, 0},
		{2, 0, 1, 1, 2},
		{2, 0, 100, 1, 2},
		{2, 1234, 0, 0, 2},
		{2, 1111111, 1, 0, 2},
		{21111111, 1111111, 1333333, 0, 2},
	}
	for _, v2 := range tests2 {
		a, b, err := cbi.GetBuckets(v2.height, v2.offset, v2.limit)
		require.NoError(err)

		var r iotextypes.VoteBucketList
		require.NoError(proto.Unmarshal(a, &r))
		require.Equal(len(r.Buckets), v2.r1)
		require.Equal(b, v2.r2)

		if len(r.Buckets) > 0 {
			require.Equal(r.Buckets[0].Index, tests[1].buckets.Buckets[0].Index)
			require.Equal(r.Buckets[0].AutoStake, tests[1].buckets.Buckets[0].AutoStake)
			require.Equal(r.Buckets[0].Owner, tests[1].buckets.Buckets[0].Owner)
		}
	}
	require.NoError(cbi.Stop(ctx))
}
