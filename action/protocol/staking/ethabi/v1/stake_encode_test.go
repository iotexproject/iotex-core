package v1

import (
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	stakingComm "github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/common"
)

// _nativeVoteBucketListEth is the expected eth-abi encoding of nativeVoteBucketList()
// through the v1 (native, 9-field) VoteBucket tuple[] output layout.
const _nativeVoteBucketListEth = "000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000007000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000006f05b59d3b20000000000000000000000000000000000000000000000000000000000000000015e000000000000000000000000000000000000000000000000000000005f5e1000000000000000000000000000000000000000000000000000000000005f5e1064000000000000000000000000000000000000000000000000000000005f5e10c8000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000c9"

func nativeVoteBucketList() *iotextypes.VoteBucketList {
	return &iotextypes.VoteBucketList{Buckets: []*iotextypes.VoteBucket{
		{
			Index:            7,
			CandidateAddress: "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqr9wrzs5u",
			StakedAmount:     "500000000000000000",
			StakedDuration:   350,
			CreateTime:       &timestamppb.Timestamp{Seconds: 1_600_000_000},
			StakeStartTime:   &timestamppb.Timestamp{Seconds: 1_600_000_100},
			UnstakeStartTime: &timestamppb.Timestamp{Seconds: 1_600_000_200},
			AutoStake:        false,
			Owner:            "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqxf907nt9",
		},
	}}
}

func mustMarshal(t *testing.T, m proto.Message) []byte {
	t.Helper()
	b, err := proto.Marshal(m)
	require.NoError(t, err)
	return b
}

// TestBucketViewsEncodeToEth exercises the EncodeToEth path (proto-unmarshal +
// eth-abi encode) and the decode error paths for every v1 vote-bucket view. The
// three views share the native VoteBucket output layout, so they must all encode
// the same list to the same bytes.
func TestBucketViewsEncodeToEth(t *testing.T) {
	cases := []struct {
		name   string
		method *abi.Method
		// input is the full eth_call data (selector + args) taken from the
		// corresponding TestBuildReadStateRequest* fixture.
		input  string
		newCtx func([]byte) (protocol.StateContext, error)
	}{
		{
			name:   "bucketsByCandidate",
			method: &_bucketsByCandidateMethod,
			input:  "387c001b000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000568656c6c6f000000000000000000000000000000000000000000000000000000",
			newCtx: func(d []byte) (protocol.StateContext, error) { return newBucketsByCandidateStateContext(d) },
		},
		{
			name:   "bucketsByIndexes",
			method: &_bucketsByIndexesMethod,
			input:  "7d141b790000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
			newCtx: func(d []byte) (protocol.StateContext, error) { return newBucketsByIndexesStateContext(d) },
		},
		{
			name:   "bucketsByVoter",
			method: &_bucketsByVoterMethod,
			input:  "4a0c59f9000000000000000000000000000000000000000000000000000000000000006400000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
			newCtx: func(d []byte) (protocol.StateContext, error) { return newBucketsByVoterStateContext(d) },
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := require.New(t)

			// method signature guard: the selector must match the call data
			r.Equal(hex.EncodeToString(c.method.ID), c.input[:8])

			data, err := hex.DecodeString(c.input)
			r.NoError(err)
			ctx, err := c.newCtx(data[4:])
			r.NoError(err)

			// well-formed payload round-trips to the expected eth encoding
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
			_, err = c.newCtx([]byte{0xde, 0xad})
			r.Error(err)
		})
	}
}
