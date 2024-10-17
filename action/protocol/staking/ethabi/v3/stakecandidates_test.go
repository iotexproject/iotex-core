package v3

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

	// data, err := _candidatesMethod.Inputs.Pack(uint32(1), uint32(2))
	// r.NoError(err)
	// data = append(_candidatesMethod.ID, data...)
	// t.Logf("data: %s", hex.EncodeToString(data))
	data, _ := hex.DecodeString("b591758b00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002")
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
			}, {
				Id:                 "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqyzm8z5y",
				OwnerAddress:       "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqyzm8z5y",
				OperatorAddress:    "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq9ldnhfk",
				RewardAddress:      "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqx37xp8f",
				Name:               "world",
				TotalWeightedVotes: "11000000000000000000",
				SelfStakeBucketIdx: 101,
				SelfStakingTokens:  "6000000000000000000",
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
	r.EqualValues("000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000040000000000000000000000000000000000000000000000000000000000000018000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000300000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000008ac7230489e8000000000000000000000000000000000000000000000000000000000000000000640000000000000000000000000000000000000000000000004563918244f400000000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000568656c6c6f000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000050000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000098a7d9b8314c0000000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000053444835ec58000000000000000000000000000000000000000000000000000000000000000000040000000000000000000000000000000000000000000000000000000000000005776f726c64000000000000000000000000000000000000000000000000000000", data)
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
