package v2

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	stakingComm "github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/common"
)

func TestBuildReadStateRequestContractBucketTypes(t *testing.T) {
	r := require.New(t)

	addr, err := address.FromString("io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqryn4k9fw")
	r.NoError(err)
	data, err := _contractBucketTypesMethod.Inputs.Pack(common.BytesToAddress(addr.Bytes()))
	r.NoError(err)
	data = append(_contractBucketTypesMethod.ID, data...)
	t.Logf("data: %s", hex.EncodeToString(data))

	data, err = hex.DecodeString("017619d40000000000000000000000000000000000000000000000000000000000000064")
	r.NoError(err)
	req, err := BuildReadStateRequest(data)
	r.NoError(err)
	r.EqualValues("*v2.ContractBucketTypesStateContext", reflect.TypeOf(req).String())

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_CONTRACT_STAKING_BUCKET_TYPES,
	}
	methodBytes, _ := proto.Marshal(method)
	r.EqualValues(methodBytes, req.Parameters().MethodName)

	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_ContractStakingBucketTypes_{
			ContractStakingBucketTypes: &iotexapi.ReadStakingDataRequest_ContractStakingBucketTypes{
				ContractAddress: "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqryn4k9fw",
			},
		},
	}
	argumentsBytes, _ := proto.Marshal(arguments)
	r.EqualValues([][]byte{argumentsBytes}, req.Parameters().Arguments)
}

func TestEncodeBucketTypeListToEth(t *testing.T) {
	r := require.New(t)

	bts := make([]*iotextypes.ContractStakingBucketType, 2)

	bts[0] = &iotextypes.ContractStakingBucketType{
		StakedAmount:   "1000000000000000000",
		StakedDuration: 1_000_000,
	}
	bts[1] = &iotextypes.ContractStakingBucketType{
		StakedAmount:   "2000000000000000000",
		StakedDuration: 2_000_000,
	}

	data, err := stakingComm.EncodeBucketTypeListToEth(_contractBucketTypesMethod.Outputs, &iotextypes.ContractStakingBucketTypeList{
		BucketTypes: bts,
	})

	r.Nil(err)
	r.EqualValues("000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000de0b6b3a764000000000000000000000000000000000000000000000000000000000000000f42400000000000000000000000000000000000000000000000001bc16d674ec8000000000000000000000000000000000000000000000000000000000000001e8480", data)
}
