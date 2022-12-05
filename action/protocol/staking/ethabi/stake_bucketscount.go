package ethabi

import (
	"encoding/hex"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/abiutil"
)

var _bucketsCountInterfaceABI = `[
	{
		"inputs": [],
		"name": "bucketsCount",
		"outputs": [
			{
				"internalType": "uint64",
				"name": "total",
				"type": "uint64"
			},
			{
				"internalType": "uint64",
				"name": "active",
				"type": "uint64"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

var _bucketsCountMethod abi.Method

func init() {
	_bucketsCountMethod = abiutil.MustLoadMethod(_bucketsCountInterfaceABI, "bucketsCount")
}

// BucketsCountStateContext context for BucketsCount
type BucketsCountStateContext struct {
	*protocol.BaseStateContext
}

func newBucketsCountStateContext() (*BucketsCountStateContext, error) {
	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_BUCKETS_COUNT,
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
		},
	}, nil
}

// EncodeToEth encode proto to eth
func (r *BucketsCountStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	var result iotextypes.BucketsCount
	if err := proto.Unmarshal(resp.Data, &result); err != nil {
		return "", err
	}

	data, err := _bucketsCountMethod.Outputs.Pack(result.Total, result.Active)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}
