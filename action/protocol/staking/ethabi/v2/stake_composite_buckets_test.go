package v2

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	stakingComm "github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/common"
)

func TestBuildReadStateRequestCompositeBuckets(t *testing.T) {
	r := require.New(t)

	// data, err := _compositeBucketsMethod.Inputs.Pack(uint32(1), uint32(5))
	// r.NoError(err)
	// data = append(_compositeBucketsMethod.ID, data...)
	// t.Logf("data: %s", hex.EncodeToString(data))

	data, _ := hex.DecodeString("40f086d600000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000005")
	req, err := BuildReadStateRequest(data)

	r.Nil(err)
	r.EqualValues("*common.BucketsStateContext", reflect.TypeOf(req).String())

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_COMPOSITE_BUCKETS,
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

func TestEncodeCompositeVoteBucketListToEth(t *testing.T) {
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
		ContractAddress:  "",
	}
	buckets[1] = &iotextypes.VoteBucket{
		Index:                     2,
		CandidateAddress:          "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqr9wrzs5u",
		StakedAmount:              "2000000000000000000",
		AutoStake:                 false,
		Owner:                     "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqxf907nt9",
		ContractAddress:           "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqxf907nt9",
		StakedDurationBlockNumber: 1_000_000,
		CreateBlockHeight:         1_000_000_000,
		StakeStartBlockHeight:     1_000_000_001,
		UnstakeStartBlockHeight:   1_000_000_002,
	}

	data, err := stakingComm.EncodeVoteBucketListToEth(_compositeBucketsMethod.Outputs, &iotextypes.VoteBucketList{
		Buckets: buckets,
	})
	t.Logf("data: %s\n", data)
	r.Nil(err)
	r.EqualValues("00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000640000000000000000000000000000000000000000000000000de0b6b3a764000000000000000000000000000000000000000000000000000000000000000f4240000000000000000000000000000000000000000000000000000000003b9aca00000000000000000000000000000000000000000000000000000000003b9aca01000000000000000000000000000000000000000000000000000000003b9aca02000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000c800000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000650000000000000000000000000000000000000000000000001bc16d674ec800000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000c900000000000000000000000000000000000000000000000000000000000000c900000000000000000000000000000000000000000000000000000000000f4240000000000000000000000000000000000000000000000000000000003b9aca00000000000000000000000000000000000000000000000000000000003b9aca01000000000000000000000000000000000000000000000000000000003b9aca02", data)
}
