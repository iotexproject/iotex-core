package v4

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/common"
)

func TestBuildReadStateRequestCandidates(t *testing.T) {
	r := require.New(t)

	data, _ := hex.DecodeString("5198e15500000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002")
	req, err := BuildReadStateRequest(data)

	r.Nil(err)
	r.EqualValues("*common.CandidatesStateContext", reflect.TypeOf(req).String())

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_CANDIDATES,
	}
	methodBytes, _ := proto.Marshal(method)
	r.EqualValues(methodBytes, req.Parameters().MethodName)

	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_Candidates_{
			Candidates: &iotexapi.ReadStakingDataRequest_Candidates{
				Pagination: &iotexapi.PaginationParam{
					Offset: 1,
					Limit:  2,
				},
			},
		},
	}
	argumentsBytes, _ := proto.Marshal(arguments)
	r.EqualValues([][]byte{argumentsBytes}, req.Parameters().Arguments)
}

func TestCandidatesToEth(t *testing.T) {
	r := require.New(t)

	candidates := &iotextypes.CandidateListV2{
		Candidates: []*iotextypes.CandidateV2{
			{
				Id:                 "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqps833xv",
				OwnerAddress:       "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqps833xv",
				OperatorAddress:    "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqz75y8gn",
				RewardAddress:      "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqrrzsj4p",
				Name:               "hello",
				TotalWeightedVotes: "10000000000000000000",
				SelfStakeBucketIdx: 100,
				SelfStakingTokens:  "5000000000000000000",
				BlsPubKey:          []byte{0, 1, 2, 3},
			}, {
				Id:                 "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqyzm8z5y",
				OwnerAddress:       "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqyzm8z5y",
				OperatorAddress:    "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq9ldnhfk",
				RewardAddress:      "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqx37xp8f",
				Name:               "world",
				TotalWeightedVotes: "11000000000000000000",
				SelfStakeBucketIdx: 101,
				SelfStakingTokens:  "6000000000000000000",
				BlsPubKey:          []byte{4, 5, 6, 7},
			},
		},
	}
	candidatesBytes, _ := proto.Marshal(candidates)
	resp := &iotexapi.ReadStateResponse{
		Data: candidatesBytes,
	}

	ctx := &common.CandidatesStateContext{
		BaseStateContext: &protocol.BaseStateContext{
			Method: &_candidatesMethod,
		},
	}
	data, err := ctx.EncodeToEth(resp)
	r.Nil(err)
	r.EqualValues("00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000001e000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000300000000000000000000000000000000000000000000000000000000000001200000000000000000000000000000000000000000000000008ac7230489e8000000000000000000000000000000000000000000000000000000000000000000640000000000000000000000000000000000000000000000004563918244f4000000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000160000000000000000000000000000000000000000000000000000000000000000568656c6c6f00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000040001020300000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000050000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000012000000000000000000000000000000000000000000000000098a7d9b8314c0000000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000053444835ec580000000000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000001600000000000000000000000000000000000000000000000000000000000000005776f726c6400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000040405060700000000000000000000000000000000000000000000000000000000", data)
}

func TestCandidatesToEthEmptyCandidates(t *testing.T) {
	r := require.New(t)

	candidates := &iotextypes.CandidateListV2{
		Candidates: []*iotextypes.CandidateV2{},
	}
	candidatesBytes, _ := proto.Marshal(candidates)
	resp := &iotexapi.ReadStateResponse{
		Data: candidatesBytes,
	}

	ctx := &common.CandidatesStateContext{
		BaseStateContext: &protocol.BaseStateContext{
			Method: &_candidatesMethod,
		},
	}
	data, err := ctx.EncodeToEth(resp)
	r.Nil(err)
	r.EqualValues("00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000", data)
}
