package v1

import (
	"encoding/hex"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	stakingComm "github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/common"
)

// _nativeVoteBucketListEth is the expected eth-abi encoding of nativeVoteBucketList()
// through the v1 (native, 9-field) VoteBucket tuple[] output layout.
const _nativeVoteBucketListEth = "00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000640000000000000000000000000000000000000000000000000de0b6b3a764000000000000000000000000000000000000000000000000000000000000000f4240000000000000000000000000000000000000000000000000000000003b9aca00000000000000000000000000000000000000000000000000000000003b9aca01000000000000000000000000000000000000000000000000000000003b9aca02000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000c8"

func nativeVoteBucketList() *iotextypes.VoteBucketList {
	return &iotextypes.VoteBucketList{Buckets: []*iotextypes.VoteBucket{
		{
			Index:            1,
			CandidateAddress: "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqryn4k9fw",
			StakedAmount:     "1000000000000000000",
			StakedDuration:   1_000_000,
			CreateTime:       &timestamppb.Timestamp{Seconds: 1_000_000_000},
			StakeStartTime:   &timestamppb.Timestamp{Seconds: 1_000_000_001},
			UnstakeStartTime: &timestamppb.Timestamp{Seconds: 1_000_000_002},
			AutoStake:        true,
			Owner:            "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqxgce2xkh",
		},
	}}
}

func mustMarshal(t *testing.T, m proto.Message) []byte {
	t.Helper()
	b, err := proto.Marshal(m)
	require.NoError(t, err)
	return b
}

func TestBucketsByCandidateEncodeToEth(t *testing.T) {
	r := require.New(t)

	// method signature guard
	r.Equal("387c001b", hex.EncodeToString(_bucketsByCandidateMethod.ID))

	// input hex from TestBuildReadStateRequestBucketsByCandidate (selector + args)
	input, _ := hex.DecodeString("387c001b000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000568656c6c6f000000000000000000000000000000000000000000000000000000")
	ctx, err := newBucketsByCandidateStateContext(input[4:])
	r.NoError(err)

	// well-formed output round-trips to the expected eth encoding
	out, err := ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: mustMarshal(t, nativeVoteBucketList())})
	r.NoError(err)
	r.Equal(_nativeVoteBucketListEth, out)

	// malformed proto payload -> unmarshal error
	_, err = ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: []byte{0xff}})
	r.Error(err)

	// non-numeric staked amount -> convert big number error
	bad := nativeVoteBucketList()
	bad.Buckets[0].StakedAmount = "not-a-number"
	_, err = ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: mustMarshal(t, bad)})
	r.ErrorIs(err, stakingComm.ErrConvertBigNumber)

	// malformed call data -> decode error
	_, err = newBucketsByCandidateStateContext([]byte{0xde, 0xad})
	r.Error(err)
}

func TestBucketsByIndexesEncodeToEth(t *testing.T) {
	r := require.New(t)

	r.Equal("7d141b79", hex.EncodeToString(_bucketsByIndexesMethod.ID))

	input, _ := hex.DecodeString("7d141b790000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002")
	ctx, err := newBucketsByIndexesStateContext(input[4:])
	r.NoError(err)

	out, err := ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: mustMarshal(t, nativeVoteBucketList())})
	r.NoError(err)
	r.Equal(_nativeVoteBucketListEth, out)

	_, err = ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: []byte{0xff}})
	r.Error(err)

	bad := nativeVoteBucketList()
	bad.Buckets[0].StakedAmount = "xx"
	_, err = ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: mustMarshal(t, bad)})
	r.ErrorIs(err, stakingComm.ErrConvertBigNumber)

	_, err = newBucketsByIndexesStateContext([]byte{0x01})
	r.Error(err)
}

func TestBucketsByVoterEncodeToEth(t *testing.T) {
	r := require.New(t)

	r.Equal("4a0c59f9", hex.EncodeToString(_bucketsByVoterMethod.ID))

	input, _ := hex.DecodeString("4a0c59f9000000000000000000000000000000000000000000000000000000000000006400000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002")
	ctx, err := newBucketsByVoterStateContext(input[4:])
	r.NoError(err)

	out, err := ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: mustMarshal(t, nativeVoteBucketList())})
	r.NoError(err)
	r.Equal(_nativeVoteBucketListEth, out)

	_, err = ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: []byte{0xff}})
	r.Error(err)

	bad := nativeVoteBucketList()
	bad.Buckets[0].StakedAmount = "xx"
	_, err = ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: mustMarshal(t, bad)})
	r.ErrorIs(err, stakingComm.ErrConvertBigNumber)

	// truncated address argument -> decode error
	_, err = newBucketsByVoterStateContext([]byte{0x01, 0x02})
	r.Error(err)
}
