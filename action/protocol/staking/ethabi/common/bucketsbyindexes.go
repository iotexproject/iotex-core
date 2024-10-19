package common

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

// BucketsByIndexesStateContext context for BucketsByIndexes
type BucketsByIndexesStateContext struct {
	*protocol.BaseStateContext
}

func NewBucketsByIndexesStateContext(data []byte, methodABI *abi.Method, apiMethod iotexapi.ReadStakingDataMethod_Name) (*BucketsByIndexesStateContext, error) {
	paramsMap := map[string]interface{}{}
	ok := false
	if err := methodABI.Inputs.UnpackIntoMap(paramsMap, data); err != nil {
		return nil, err
	}
	var index []uint64
	if index, ok = paramsMap["indexes"].([]uint64); !ok {
		return nil, ErrDecodeFailure
	}

	method := &iotexapi.ReadStakingDataMethod{
		Method: apiMethod,
	}
	methodBytes, err := proto.Marshal(method)
	if err != nil {
		return nil, err
	}
	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketsByIndexes{
			BucketsByIndexes: &iotexapi.ReadStakingDataRequest_VoteBucketsByIndexes{
				Index: index,
			},
		},
	}
	argumentsBytes, err := proto.Marshal(arguments)
	if err != nil {
		return nil, err
	}
	return &BucketsByIndexesStateContext{
		&protocol.BaseStateContext{
			Parameter: &protocol.Parameters{
				MethodName: methodBytes,
				Arguments:  [][]byte{argumentsBytes},
			},
			Method: methodABI,
		},
	}, nil
}

// EncodeToEth encode proto to eth
func (r *BucketsByIndexesStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	var result iotextypes.VoteBucketList
	if err := proto.Unmarshal(resp.Data, &result); err != nil {
		return "", err
	}

	return EncodeVoteBucketListToEth(r.Method.Outputs, &result)
}
