package v3

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestBuildReadStateRequestCompositeBucketsByVoter(t *testing.T) {
	r := require.New(t)

	// addr, err := address.FromString("io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqryn4k9fw")
	// r.NoError(err)
	// data, err := _compositeBucketsByVoterMethod.Inputs.Pack(common.BytesToAddress(addr.Bytes()), uint32(1), uint32(2))
	// r.NoError(err)
	// data = append(_compositeBucketsByVoterMethod.ID, data...)
	// t.Logf("data: %s", hex.EncodeToString(data))

	data, _ := hex.DecodeString("2736514b000000000000000000000000000000000000000000000000000000000000006400000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002")
	req, err := BuildReadStateRequest(data)

	r.Nil(err)
	r.EqualValues("*common.BucketsByVoterStateContext", reflect.TypeOf(req).String())

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_COMPOSITE_BUCKETS_BY_VOTER,
	}
	methodBytes, _ := proto.Marshal(method)
	r.EqualValues(methodBytes, req.Parameters().MethodName)

	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketsByVoter{
			BucketsByVoter: &iotexapi.ReadStakingDataRequest_VoteBucketsByVoter{
				VoterAddress: "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqryn4k9fw",
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
