package ethabi

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestBuildReadStateRequestBuckets(t *testing.T) {
	r := require.New(t)

	data, _ := hex.DecodeString("b1ff5c2400000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000005")
	req, err := BuildReadStateRequest(data)

	r.Nil(err)
	r.EqualValues("*ethabi.BucketsStateContext", reflect.TypeOf(req).String())

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_BUCKETS,
	}
	methodBytes, _ := proto.Marshal(method)
	r.EqualValues(methodBytes, req.Parameters().MethodName)

	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_Buckets{
			Buckets: &iotexapi.ReadStakingDataRequest_VoteBuckets{
				Pagination: &iotexapi.PaginationParam{
					Offset: 1,
					Limit:  5,
				},
			},
		},
	}
	argumentsBytes, _ := proto.Marshal(arguments)
	r.EqualValues([][]byte{argumentsBytes}, req.Parameters().Arguments)
}

func TestEncodeVoteBucketListToEth(t *testing.T) {
	r := require.New(t)

	buckets := make([]*iotextypes.VoteBucket, 2)

	buckets[0] = &iotextypes.VoteBucket{
		Index:            1,
		CandidateAddress: "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqryn4k9fw",
		StakedAmount:     "1000000000000000000",
		StakedDuration:   1_000_000,
		CreateTime:       &timestamppb.Timestamp{Seconds: 1_000_000_000},
		StakeStartTime:   &timestamppb.Timestamp{Seconds: 1_000_000_001},
		UnstakeStartTime: &timestamppb.Timestamp{Seconds: 1_000_000_002},
		AutoStake:        true,
		Owner:            "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqxgce2xkh",
	}
	buckets[1] = &iotextypes.VoteBucket{
		Index:            2,
		CandidateAddress: "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqr9wrzs5u",
		StakedAmount:     "2000000000000000000",
		StakedDuration:   2_000_000,
		CreateTime:       &timestamppb.Timestamp{Seconds: 1_000_000_100},
		StakeStartTime:   &timestamppb.Timestamp{Seconds: 1_000_000_101},
		UnstakeStartTime: &timestamppb.Timestamp{Seconds: 1_000_000_102},
		AutoStake:        false,
		Owner:            "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqxf907nt9",
	}

	data, err := encodeVoteBucketListToEth(_bucketsMethod.Outputs, iotextypes.VoteBucketList{
		Buckets: buckets,
	})

	r.Nil(err)
	r.EqualValues("00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000640000000000000000000000000000000000000000000000000de0b6b3a764000000000000000000000000000000000000000000000000000000000000000f4240000000000000000000000000000000000000000000000000000000003b9aca00000000000000000000000000000000000000000000000000000000003b9aca01000000000000000000000000000000000000000000000000000000003b9aca02000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000c8000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000650000000000000000000000000000000000000000000000001bc16d674ec8000000000000000000000000000000000000000000000000000000000000001e8480000000000000000000000000000000000000000000000000000000003b9aca64000000000000000000000000000000000000000000000000000000003b9aca65000000000000000000000000000000000000000000000000000000003b9aca66000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000c9", data)
}

func TestEncodeVoteBucketListToEthEmptyBuckets(t *testing.T) {
	r := require.New(t)

	buckets := make([]*iotextypes.VoteBucket, 0)

	data, err := encodeVoteBucketListToEth(_bucketsMethod.Outputs, iotextypes.VoteBucketList{
		Buckets: buckets,
	})

	r.Nil(err)
	r.EqualValues("00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000", data)
}

func TestEncodeVoteBucketListToEthErrorCandidateAddress(t *testing.T) {
	r := require.New(t)

	buckets := make([]*iotextypes.VoteBucket, 1)

	buckets[0] = &iotextypes.VoteBucket{
		Index:            1,
		CandidateAddress: "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqryn4k9f",
		StakedAmount:     "1000000000000000000",
		StakedDuration:   1_000_000,
		CreateTime:       &timestamppb.Timestamp{Seconds: 1_000_000_000},
		StakeStartTime:   &timestamppb.Timestamp{Seconds: 1_000_000_001},
		UnstakeStartTime: &timestamppb.Timestamp{Seconds: 1_000_000_002},
		AutoStake:        true,
		Owner:            "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqxgce2xkh",
	}

	_, err := encodeVoteBucketListToEth(_bucketsMethod.Outputs, iotextypes.VoteBucketList{
		Buckets: buckets,
	})

	r.EqualValues("address length = 40, expecting 41: invalid address", err.Error())
}

func TestEncodeVoteBucketListToEthErrorStakedAmount(t *testing.T) {
	r := require.New(t)

	buckets := make([]*iotextypes.VoteBucket, 1)

	buckets[0] = &iotextypes.VoteBucket{
		Index:            1,
		CandidateAddress: "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqryn4k9fw",
		StakedAmount:     "xxx",
		StakedDuration:   1_000_000,
		CreateTime:       &timestamppb.Timestamp{Seconds: 1_000_000_000},
		StakeStartTime:   &timestamppb.Timestamp{Seconds: 1_000_000_001},
		UnstakeStartTime: &timestamppb.Timestamp{Seconds: 1_000_000_002},
		AutoStake:        true,
		Owner:            "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqxgce2xkh",
	}

	_, err := encodeVoteBucketListToEth(_bucketsMethod.Outputs, iotextypes.VoteBucketList{
		Buckets: buckets,
	})

	r.EqualValues("convert big number error", err.Error())
}

func TestEncodeVoteBucketListToEthErrorOwner(t *testing.T) {
	r := require.New(t)

	buckets := make([]*iotextypes.VoteBucket, 1)

	buckets[0] = &iotextypes.VoteBucket{
		Index:            1,
		CandidateAddress: "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqryn4k9fw",
		StakedAmount:     "1000000000000000000",
		StakedDuration:   1_000_000,
		CreateTime:       &timestamppb.Timestamp{Seconds: 1_000_000_000},
		StakeStartTime:   &timestamppb.Timestamp{Seconds: 1_000_000_001},
		UnstakeStartTime: &timestamppb.Timestamp{Seconds: 1_000_000_002},
		AutoStake:        true,
		Owner:            "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqxgce2xk",
	}

	_, err := encodeVoteBucketListToEth(_bucketsMethod.Outputs, iotextypes.VoteBucketList{
		Buckets: buckets,
	})

	r.EqualValues("address length = 40, expecting 41: invalid address", err.Error())
}
