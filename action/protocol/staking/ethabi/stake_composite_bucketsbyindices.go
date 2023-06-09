package ethabi

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/abiutil"
)

const _compositeBucketsByIndexesInterfaceABI = `[
	{
		"inputs": [
			{
				"internalType": "uint64[]",
				"name": "indexes",
				"type": "uint64[]"
			}
		],
		"name": "compositeBucketsByIndexes",
		"outputs": [
			{
				"components": [
					{
						"internalType": "uint64",
						"name": "index",
						"type": "uint64"
					},
					{
						"internalType": "address",
						"name": "candidateAddress",
						"type": "address"
					},
					{
						"internalType": "uint256",
						"name": "stakedAmount",
						"type": "uint256"
					},
					{
						"internalType": "uint32",
						"name": "stakedDuration",
						"type": "uint32"
					},
					{
						"internalType": "int64",
						"name": "createTime",
						"type": "int64"
					},
					{
						"internalType": "int64",
						"name": "stakeStartTime",
						"type": "int64"
					},
					{
						"internalType": "int64",
						"name": "unstakeStartTime",
						"type": "int64"
					},
					{
						"internalType": "bool",
						"name": "autoStake",
						"type": "bool"
					},
					{
						"internalType": "address",
						"name": "owner",
						"type": "address"
					},
					{
						"internalType": "address",
						"name": "contractAddress",
						"type": "address"
					},
					{
						"internalType": "uint64",
						"name": "stakedDurationBlockNumber",
						"type": "uint64"
					},
					{
						"internalType": "uint64",
						"name": "createBlockHeight",
						"type": "uint64"
					},
					{
						"internalType": "uint64",
						"name": "stakeStartBlockHeight",
						"type": "uint64"
					},
					{
						"internalType": "uint64",
						"name": "unstakeStartBlockHeight",
						"type": "uint64"
					}
				],
				"internalType": "struct IStaking.CompositeVoteBucket[]",
				"name": "",
				"type": "tuple[]"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

var _compositeBucketsByIndexesMethod abi.Method

func init() {
	_compositeBucketsByIndexesMethod = abiutil.MustLoadMethod(_compositeBucketsByIndexesInterfaceABI, "compositeBucketsByIndexes")
}

// CompositeBucketsByIndexesStateContext context for CompositeBucketsByIndexes
type CompositeBucketsByIndexesStateContext struct {
	*protocol.BaseStateContext
}

func newCompositeBucketsByIndexesStateContext(data []byte) (*CompositeBucketsByIndexesStateContext, error) {
	paramsMap := map[string]interface{}{}
	ok := false
	if err := _compositeBucketsByIndexesMethod.Inputs.UnpackIntoMap(paramsMap, data); err != nil {
		return nil, err
	}
	var index []uint64
	if index, ok = paramsMap["indexes"].([]uint64); !ok {
		return nil, errDecodeFailure
	}

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_COMPOSITE_BUCKETS_BY_INDEXES,
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
	return &CompositeBucketsByIndexesStateContext{
		&protocol.BaseStateContext{
			Parameter: &protocol.Parameters{
				MethodName: methodBytes,
				Arguments:  [][]byte{argumentsBytes},
			},
		},
	}, nil
}

// EncodeToEth encode proto to eth
func (r *CompositeBucketsByIndexesStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	var result iotextypes.VoteBucketList
	if err := proto.Unmarshal(resp.Data, &result); err != nil {
		return "", err
	}

	return encodeVoteBucketListToEth(_compositeBucketsByIndexesMethod.Outputs, result)
}
