package v3

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestBuildReadStateRequestCompositeBucketsByCandidate(t *testing.T) {
	r := require.New(t)

	// data, err := _compositeBucketsByCandidateMethod.Inputs.Pack("hello", uint32(0), uint32(1))
	// r.NoError(err)
	// data = append(_compositeBucketsByCandidateMethod.ID, data...)
	// t.Logf("data: %s", hex.EncodeToString(data))

	data, _ := hex.DecodeString("71a3e9b9000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000568656c6c6f000000000000000000000000000000000000000000000000000000")
	req, err := BuildReadStateRequest(data)

	r.Nil(err)
	r.EqualValues("*common.BucketsByCandidateStateContext", reflect.TypeOf(req).String())

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_COMPOSITE_BUCKETS_BY_CANDIDATE,
	}
	methodBytes, _ := proto.Marshal(method)
	r.EqualValues(methodBytes, req.Parameters().MethodName)

	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketsByCandidate{
			BucketsByCandidate: &iotexapi.ReadStakingDataRequest_VoteBucketsByCandidate{
				CandName: "hello",
				Pagination: &iotexapi.PaginationParam{
					Offset: 0,
					Limit:  1,
				},
			},
		},
	}
	argumentsBytes, _ := proto.Marshal(arguments)
	r.EqualValues([][]byte{argumentsBytes}, req.Parameters().Arguments)
}
