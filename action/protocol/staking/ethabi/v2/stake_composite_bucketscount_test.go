package v2

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

func TestBuildReadStateRequestCompositeBucketsCount(t *testing.T) {
	r := require.New(t)

	data, _ := hex.DecodeString("a40d6b8c")
	req, err := BuildReadStateRequest(data)

	r.Nil(err)
	r.EqualValues("*common.BucketsCountStateContext", reflect.TypeOf(req).String())

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_COMPOSITE_BUCKETS_COUNT,
	}
	methodBytes, _ := proto.Marshal(method)
	r.EqualValues(methodBytes, req.Parameters().MethodName)

	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketsCount_{
			BucketsCount: &iotexapi.ReadStakingDataRequest_BucketsCount{},
		},
	}
	argumentsBytes, _ := proto.Marshal(arguments)
	r.EqualValues([][]byte{argumentsBytes}, req.Parameters().Arguments)
}

func TestEncodeCompositeBucketsCountToEth(t *testing.T) {
	r := require.New(t)

	count := &iotextypes.BucketsCount{
		Total:  5,
		Active: 2,
	}
	countBytes, _ := proto.Marshal(count)
	resp := &iotexapi.ReadStateResponse{
		Data: countBytes,
	}

	ctx := &common.BucketsCountStateContext{
		BaseStateContext: &protocol.BaseStateContext{
			Method: &_compositeBucketsCountMethod,
		},
	}
	data, err := ctx.EncodeToEth(resp)
	r.Nil(err)
	r.EqualValues("00000000000000000000000000000000000000000000000000000000000000050000000000000000000000000000000000000000000000000000000000000002", data)
}
