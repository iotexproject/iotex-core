package staking

import (
	"context"
	"testing"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/testutil"
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

	require.EqualError(cbi.PutCandidates(0, nil), "proto: Marshal called with nil")

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

	//incorrect parameter
	tests2 := []struct {
		height        uint64
		offset, limit uint32
	}{
		{0, 0, 0},
		{2, 1111111, 1},
		{21111111, 1111111, 1333333},
	}
	for _, v2 := range tests2 {
		_, _, err = cbi.GetCandidates(v2.height, v2.offset, v2.limit)
		require.NoError(err)
	}

	b, h, err := cbi.GetCandidates(tests[1].height, 0, 1)
	require.NoError(err)
	require.Equal(h, tests[1].height)

	var r iotextypes.CandidateListV2
	require.NoError(proto.Unmarshal(b, &r))
	require.Equal(len(r.Candidates), 1)
	require.Equal(r.Candidates[0].Name, tests[1].candidates.Candidates[0].Name)
	require.Equal(r.Candidates[0].SelfStakeBucketIdx, tests[1].candidates.Candidates[0].SelfStakeBucketIdx)

	require.NoError(cbi.Stop(ctx))

	//get candidates after stop kv db
	path := "test-kv-store.bolt"
	testPath, err := testutil.PathOfTempFile(path)
	require.NoError(err)
	cfg := config.Default.DB
	cfg.DbPath = testPath

	store = db.NewBoltDB(cfg)
	cbi, err = NewStakingCandidatesBucketsIndexer(store)
	require.NoError(err)

	require.NoError(cbi.Start(ctx))
	require.NoError(cbi.Stop(ctx))
	_, _, err = cbi.GetCandidates(tests[1].height, 0, 1)
	require.EqualError(err, "database not open: DB I/O operation error")

	testutil.CleanupPath(t, testPath)
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

	require.EqualError(cbi.PutBuckets(0, nil), "proto: Marshal called with nil")

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

	//incorrect parameter
	tests2 := []struct {
		height        uint64
		offset, limit uint32
	}{
		{0, 0, 0},
		{2, 1111111, 1},
		{21111111, 1111111, 1333333},
	}
	for _, v2 := range tests2 {
		_, _, err = cbi.GetBuckets(v2.height, v2.offset, v2.limit)
		require.NoError(err)
	}

	b, h, err := cbi.GetBuckets(tests[1].height, 0, 1)
	require.NoError(err)
	require.Equal(h, uint64(2))

	var r iotextypes.VoteBucketList
	require.NoError(proto.Unmarshal(b, &r))
	require.Equal(len(r.Buckets), 1)
	require.Equal(r.Buckets[0].Index, tests[1].buckets.Buckets[0].Index)
	require.Equal(r.Buckets[0].AutoStake, tests[1].buckets.Buckets[0].AutoStake)
	require.Equal(r.Buckets[0].Owner, tests[1].buckets.Buckets[0].Owner)

	require.NoError(cbi.Stop(ctx))
}
