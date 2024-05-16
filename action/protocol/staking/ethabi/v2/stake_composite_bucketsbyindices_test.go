package v2

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestBuildReadStateRequestCompositeBucketsByIndexes(t *testing.T) {
	r := require.New(t)

	// data, err := _compositeBucketsByIndexesMethod.Inputs.Pack([]uint64{1, 2})
	// r.NoError(err)
	// data = append(_compositeBucketsByIndexesMethod.ID, data...)
	// t.Logf("data: %s", hex.EncodeToString(data))

	data, _ := hex.DecodeString("347cdbd50000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002")
	req, err := BuildReadStateRequest(data)

	r.Nil(err)
	r.EqualValues("*common.BucketsByIndexesStateContext", reflect.TypeOf(req).String())

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_COMPOSITE_BUCKETS_BY_INDEXES,
	}
	methodBytes, _ := proto.Marshal(method)
	r.EqualValues(methodBytes, req.Parameters().MethodName)

	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketsByIndexes{
			BucketsByIndexes: &iotexapi.ReadStakingDataRequest_VoteBucketsByIndexes{
				Index: []uint64{1, 2},
			},
		},
	}
	argumentsBytes, _ := proto.Marshal(arguments)
	r.EqualValues([][]byte{argumentsBytes}, req.Parameters().Arguments)
}
