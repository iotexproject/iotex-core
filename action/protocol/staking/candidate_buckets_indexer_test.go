package staking

import (
	"context"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestCandidatesBucketsIndexer_PutGetCandidates(t *testing.T) {
	require := require.New(t)

	testPath, err := testutil.PathOfTempFile("test-candidate")
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, testPath)
	}()

	cfg := db.DefaultConfig
	cfg.DbPath = testPath
	store := db.NewBoltDB(cfg)
	cbi, err := NewStakingCandidatesBucketsIndexer(store)
	require.NoError(err)
	ctx := context.Background()
	require.NoError(cbi.Start(ctx))
	defer func() {
		require.NoError(cbi.Stop(ctx))
	}()

	cand := []*iotextypes.CandidateV2{
		{
			OwnerAddress:       "owner1",
			Name:               "abc",
			SelfStakeBucketIdx: 123,
		},
	}
	cand2 := &iotextypes.CandidateListV2{
		Candidates: cand,
	}
	cand3 := &iotextypes.CandidateListV2{
		Candidates: append(cand, &iotextypes.CandidateV2{
			OwnerAddress:       "owner2",
			Name:               "xyz",
			SelfStakeBucketIdx: 456,
		}),
	}

	tests := []struct {
		height     uint64
		candidates *iotextypes.CandidateListV2
	}{
		{0, nil},
		{1, &iotextypes.CandidateListV2{}},
		{2, cand2},
		{3, cand3},
		{4, cand3}, // same as block 3
		{5, cand3}, // same as block 3
		{6, cand2}, // same as block 2
	}
	tests2 := []struct {
		offset, limit uint32
	}{
		{0, 1},
		{0, 2},
		{1, 5},
		{1234, 5},
	}

	var r iotextypes.CandidateListV2
	for _, v := range tests {
		require.NoError(cbi.PutCandidates(v.height, v.candidates))

		for _, v2 := range tests2 {
			a, b, err := cbi.GetCandidates(v.height, v2.offset, v2.limit)
			require.NoError(err)
			require.Equal(b, v.height)
			require.NoError(proto.Unmarshal(a, &r))
			if tests[v.height].candidates == nil {
				continue
			}
			expectLen := uint32(len(tests[v.height].candidates.Candidates))
			if expectLen == 0 || v2.offset >= expectLen {
				require.Zero(len(r.Candidates))
				continue
			}
			end := v2.offset + v2.limit
			if end > expectLen {
				end = expectLen
			}
			// check the returned list
			expect := tests[v.height].candidates.Candidates[v2.offset:end]
			for i, v := range expect {
				actual := r.Candidates[i]
				require.Equal(v.OwnerAddress, actual.OwnerAddress)
				require.Equal(v.Name, actual.Name)
				require.Equal(v.SelfStakeBucketIdx, actual.SelfStakeBucketIdx)
			}
		}
	}

	// test height > latest height
	require.EqualValues(6, cbi.latestCandidatesHeight)
	a, b, err := cbi.GetCandidates(7, 0, 1)
	require.NoError(err)
	require.EqualValues(6, b)
	require.NoError(proto.Unmarshal(a, &r))
	require.Equal(1, len(r.Candidates))
	expect := tests[6].candidates.Candidates[0]
	actual := r.Candidates[0]
	require.Equal(expect.OwnerAddress, actual.OwnerAddress)
	require.Equal(expect.Name, actual.Name)
	require.Equal(expect.SelfStakeBucketIdx, actual.SelfStakeBucketIdx)

	// test with a key larger than any existing key
	height := uint64(8810200527999860736)
	candMax := &iotextypes.CandidateListV2{
		Candidates: append(cand, &iotextypes.CandidateV2{
			OwnerAddress:       "ownermax",
			Name:               "alphabeta",
			SelfStakeBucketIdx: 789,
		}),
	}
	require.NoError(cbi.PutCandidates(height, candMax))
	lcHash := cbi.latestCandidatesHash
	a, _, err = cbi.GetCandidates(height, 0, 4)
	require.NoError(err)
	c, err := getFromIndexer(store, StakingCandidatesNamespace, height+1)
	require.NoError(err)
	require.Equal(a, c)
	require.NoError(cbi.Stop(ctx))

	// reopen db to read latest height and hash
	require.NoError(cbi.Start(ctx))
	require.Equal(height, cbi.latestCandidatesHeight)
	require.Equal(lcHash, cbi.latestCandidatesHash)
}

func TestCandidatesBucketsIndexer_PutGetBuckets(t *testing.T) {
	require := require.New(t)

	testPath, err := testutil.PathOfTempFile("test-bucket")
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, testPath)
	}()

	cfg := db.DefaultConfig
	cfg.DbPath = testPath
	store := db.NewBoltDB(cfg)
	cbi, err := NewStakingCandidatesBucketsIndexer(store)
	require.NoError(err)
	ctx := context.Background()
	require.NoError(cbi.Start(ctx))
	defer func() {
		require.NoError(cbi.Stop(ctx))
	}()

	bucket := []*iotextypes.VoteBucket{
		{
			Index:     uint64(1234),
			AutoStake: true,
			Owner:     "abc",
		},
	}
	vote2 := &iotextypes.VoteBucketList{
		Buckets: bucket,
	}
	vote4 := &iotextypes.VoteBucketList{
		Buckets: append(bucket, &iotextypes.VoteBucket{
			Index:     uint64(5678),
			AutoStake: false,
			Owner:     "xyz",
		}),
	}

	tests := []struct {
		height  uint64
		buckets *iotextypes.VoteBucketList
	}{
		{0, nil},
		{1, &iotextypes.VoteBucketList{}},
		{2, vote2},
		{3, vote2}, // same as block 2
		{4, vote4},
		{5, vote4}, // same as block 4
		{6, vote2}, // same as block 2
	}
	tests2 := []struct {
		offset, limit uint32
	}{
		{0, 1},
		{0, 2},
		{1, 5},
		{1234, 5},
	}

	var r iotextypes.VoteBucketList
	for _, v := range tests {
		require.NoError(cbi.PutBuckets(v.height, v.buckets))

		for _, v2 := range tests2 {
			a, b, err := cbi.GetBuckets(v.height, v2.offset, v2.limit)
			require.NoError(err)
			require.Equal(b, v.height)
			require.NoError(proto.Unmarshal(a, &r))
			if tests[v.height].buckets == nil {
				continue
			}
			expectLen := uint32(len(tests[v.height].buckets.Buckets))
			if expectLen == 0 || v2.offset >= expectLen {
				require.Zero(len(r.Buckets))
				continue
			}
			end := v2.offset + v2.limit
			if end > expectLen {
				end = expectLen
			}
			// check the returned list
			expect := tests[v.height].buckets.Buckets[v2.offset:end]
			for i, v := range expect {
				actual := r.Buckets[i]
				require.Equal(v.Index, actual.Index)
				require.Equal(v.AutoStake, actual.AutoStake)
				require.Equal(v.Owner, actual.Owner)
			}
		}
	}

	// test height > latest height
	require.EqualValues(6, cbi.latestBucketsHeight)
	a, b, err := cbi.GetBuckets(7, 0, 1)
	require.NoError(err)
	require.EqualValues(6, b)
	require.NoError(proto.Unmarshal(a, &r))
	require.Equal(1, len(r.Buckets))
	expect := tests[6].buckets.Buckets[0]
	actual := r.Buckets[0]
	require.Equal(expect.Index, actual.Index)
	require.Equal(expect.AutoStake, actual.AutoStake)
	require.Equal(expect.Owner, actual.Owner)

	// test with a key larger than any existing key
	height := uint64(8810200527999860736)
	voteMax := &iotextypes.VoteBucketList{
		Buckets: append(bucket, &iotextypes.VoteBucket{
			Index:     uint64(1357),
			AutoStake: false,
			Owner:     "ownermax",
		}),
	}
	require.NoError(cbi.PutBuckets(height, voteMax))
	lbHash := cbi.latestBucketsHash
	a, _, err = cbi.GetBuckets(height, 0, 4)
	require.NoError(err)
	c, err := getFromIndexer(store, StakingBucketsNamespace, height+1)
	require.NoError(err)
	require.Equal(a, c)
	require.NoError(cbi.Stop(ctx))

	// reopen db to read latest height and hash
	require.NoError(cbi.Start(ctx))
	require.Equal(height, cbi.latestBucketsHeight)
	require.Equal(lbHash, cbi.latestBucketsHash)
}
