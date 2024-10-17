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

func TestBuildReadStateRequestCandidateByName(t *testing.T) {
	r := require.New(t)

	// data, err := _candidateByNameMethod.Inputs.Pack("hello")
	// r.NoError(err)
	// data = append(_candidateByNameMethod.ID, data...)
	// t.Logf("data: %s", hex.EncodeToString(data))

	data, _ := hex.DecodeString("a81707940000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000568656c6c6f000000000000000000000000000000000000000000000000000000")
	req, err := BuildReadStateRequest(data)

	r.Nil(err)
	r.EqualValues("*common.CandidateByNameStateContext", reflect.TypeOf(req).String())

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_CANDIDATE_BY_NAME,
	}
	methodBytes, _ := proto.Marshal(method)
	r.EqualValues(methodBytes, req.Parameters().MethodName)

	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_CandidateByName_{
			CandidateByName: &iotexapi.ReadStakingDataRequest_CandidateByName{
				CandName: "hello",
			},
		},
	}
	argumentsBytes, _ := proto.Marshal(arguments)
	r.EqualValues([][]byte{argumentsBytes}, req.Parameters().Arguments)
}

func TestCandidateByNameToEth(t *testing.T) {
	r := require.New(t)

	candidate := &iotextypes.CandidateV2{
		Id:                 "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqps833xv",
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

	ctx := &common.CandidateByNameStateContext{
		BaseStateContext: &protocol.BaseStateContext{
			Method: &_candidateByNameMethod,
		},
	}
	data, err := ctx.EncodeToEth(resp)
	r.Nil(err)
	r.EqualValues("000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000300000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000008ac7230489e8000000000000000000000000000000000000000000000000000000000000000000640000000000000000000000000000000000000000000000004563918244f400000000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000568656c6c6f000000000000000000000000000000000000000000000000000000", data)
}
