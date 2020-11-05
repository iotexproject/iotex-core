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

	b, h, err := cbi.GetCandidates(2, 0, 1)
	require.NoError(err)
	require.Equal(h, uint64(2))

	var r iotextypes.CandidateListV2
	require.NoError(proto.Unmarshal(b, &r))
	require.Equal(len(r.Candidates), 1)
	require.Equal(r.Candidates[0].Name, "abc")
	require.Equal(r.Candidates[0].SelfStakeBucketIdx, uint64(123))

	require.NoError(cbi.Stop(ctx))
}
