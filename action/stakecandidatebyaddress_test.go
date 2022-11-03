package action

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestCallDataToStakeStateContextCandidateByAddress(t *testing.T) {
	r := require.New(t)

	data, _ := hex.DecodeString("43f75ae40000000000000000000000000000000000000000000000000000000000000001")
	req, err := CallDataToStakeStateContext(data)

	r.Nil(err)
	r.EqualValues("*action.CandidateByAddressStateContext", reflect.TypeOf(req).String())

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_CANDIDATE_BY_ADDRESS,
	}
	methodBytes, _ := proto.Marshal(method)
	r.EqualValues(methodBytes, req.Parameters().MethodName)

	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_CandidateByAddress_{
			CandidateByAddress: &iotexapi.ReadStakingDataRequest_CandidateByAddress{
				OwnerAddr: "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqps833xv",
			},
		},
	}
	argumentsBytes, _ := proto.Marshal(arguments)
	r.EqualValues([][]byte{argumentsBytes}, req.Parameters().Arguments)
}

func TestCandidateByAddressToEth(t *testing.T) {
	r := require.New(t)

	candidate := &iotextypes.CandidateV2{
		OwnerAddress:       "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqps833xv",
		OperatorAddress:    "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqz75y8gn",
		RewardAddress:      "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqrrzsj4p",
		Name:               "hello",
		TotalWeightedVotes: "10000000000000000000",
		SelfStakeBucketIdx: 100,
		SelfStakingTokens:  "5000000000000000000",
	}

	candidateBytes, _ := proto.Marshal(candidate)
	resp := &iotexapi.ReadStateResponse{
		Data: candidateBytes,
	}

	ctx := &CandidateByAddressStateContext{}
	data, err := ctx.EncodeToEth(resp)
	r.Nil(err)
	r.EqualValues("000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000300000000000000000000000000000000000000000000000000000000000000e00000000000000000000000000000000000000000000000008ac7230489e8000000000000000000000000000000000000000000000000000000000000000000640000000000000000000000000000000000000000000000004563918244f40000000000000000000000000000000000000000000000000000000000000000000568656c6c6f000000000000000000000000000000000000000000000000000000", data)
}
