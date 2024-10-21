package common

import (
	"encoding/hex"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

// BucketsCountStateContext context for BucketsCount
type BucketsCountStateContext struct {
	*protocol.BaseStateContext
}

func NewBucketsCountStateContext(data []byte, methodABI *abi.Method, apiMethod iotexapi.ReadStakingDataMethod_Name) (*BucketsCountStateContext, error) {
	method := &iotexapi.ReadStakingDataMethod{
		Method: apiMethod,
	}
	methodBytes, err := proto.Marshal(method)
	if err != nil {
		return nil, err
	}
	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketsCount_{
			BucketsCount: &iotexapi.ReadStakingDataRequest_BucketsCount{},
		},
	}
	argumentsBytes, err := proto.Marshal(arguments)
	if err != nil {
		return nil, err
	}
	return &BucketsCountStateContext{
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
func (r *BucketsCountStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	var result iotextypes.BucketsCount
	if err := proto.Unmarshal(resp.Data, &result); err != nil {
		return "", err
	}

	data, err := r.Method.Outputs.Pack(result.Total, result.Active)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}
