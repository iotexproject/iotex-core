package ethabi

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestBuildReadStateRequestBucketsCount(t *testing.T) {
	r := require.New(t)

	data, _ := hex.DecodeString("fa87b185")
	req, err := BuildReadStateRequest(data)

	r.Nil(err)
	r.EqualValues("*ethabi.BucketsCountStateContext", reflect.TypeOf(req).String())

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_BUCKETS_COUNT,
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

func TestEncodeBucketsCountToEth(t *testing.T) {
	r := require.New(t)

	count := &iotextypes.BucketsCount{
		Total:  5,
		Active: 2,
	}
	countBytes, _ := proto.Marshal(count)
	resp := &iotexapi.ReadStateResponse{
		Data: countBytes,
	}

	ctx := &BucketsCountStateContext{}
	data, err := ctx.EncodeToEth(resp)
	r.Nil(err)
	r.EqualValues("00000000000000000000000000000000000000000000000000000000000000050000000000000000000000000000000000000000000000000000000000000002", data)
}
