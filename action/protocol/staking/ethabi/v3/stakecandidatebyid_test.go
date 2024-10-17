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
	stakingComm "github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/common"
)

func TestBuildReadStateRequestCandidateByID(t *testing.T) {
	r := require.New(t)

	// addr, err := address.FromString("io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqps833xv")
	// r.NoError(err)
	// data, err := _candidateByIDMethod.Inputs.Pack(common.BytesToAddress(addr.Bytes()))
	// r.NoError(err)
	// data = append(_candidateByIDMethod.ID, data...)
	// t.Logf("data: %s", hex.EncodeToString(data))

	data, _ := hex.DecodeString("794368820000000000000000000000000000000000000000000000000000000000000001")
	req, err := BuildReadStateRequest(data)

	r.Nil(err)
	r.EqualValues("*common.CandidateByAddressStateContext", reflect.TypeOf(req).String())

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_CANDIDATE_BY_ADDRESS,
	}
	methodBytes, _ := proto.Marshal(method)
	r.EqualValues(methodBytes, req.Parameters().MethodName)

	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_CandidateByAddress_{
			CandidateByAddress: &iotexapi.ReadStakingDataRequest_CandidateByAddress{
				Id: "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqps833xv",
			},
		},
	}
	argumentsBytes, _ := proto.Marshal(arguments)
	r.EqualValues([][]byte{argumentsBytes}, req.Parameters().Arguments)
}

func TestCandidateByIDToEth(t *testing.T) {
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

	ctx := &stakingComm.CandidateByAddressStateContext{
		BaseStateContext: &protocol.BaseStateContext{
			Method: &_candidateByIDMethod,
		},
	}
	data, err := ctx.EncodeToEth(resp)
	r.Nil(err)
	r.EqualValues("000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000300000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000008ac7230489e8000000000000000000000000000000000000000000000000000000000000000000640000000000000000000000000000000000000000000000004563918244f400000000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000568656c6c6f000000000000000000000000000000000000000000000000000000", data)
}
