package v3

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/abiutil"
	stakingComm "github.com/iotexproject/iotex-core/action/protocol/staking/ethabi/common"
)

const _compositeBucketsInterfaceABI = `[
	{
		"inputs": [
			{
				"internalType": "uint32",
				"name": "offset",
				"type": "uint32"
			},
			{
				"internalType": "uint32",
				"name": "limit",
				"type": "uint32"
			}
		],
		"name": "compositeBucketsV3",
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
					},
					{
						"internalType": "uint64",
						"name": "endorsementExpireBlockHeight",
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

var _compositeBucketsMethod abi.Method

func init() {
	_compositeBucketsMethod = abiutil.MustLoadMethod(_compositeBucketsInterfaceABI, "compositeBucketsV3")
}

// CompositeBucketsStateContext context for Composite Buckets
type CompositeBucketsStateContext struct {
	*protocol.BaseStateContext
}

func newCompositeBucketsStateContext(data []byte) (*CompositeBucketsStateContext, error) {
	paramsMap := map[string]interface{}{}
	ok := false
	if err := _compositeBucketsMethod.Inputs.UnpackIntoMap(paramsMap, data); err != nil {
		return nil, err
	}
	var offset, limit uint32
	if offset, ok = paramsMap["offset"].(uint32); !ok {
		return nil, stakingComm.ErrDecodeFailure
	}
	if limit, ok = paramsMap["limit"].(uint32); !ok {
		return nil, stakingComm.ErrDecodeFailure
	}

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_COMPOSITE_BUCKETS,
	}
	methodBytes, err := proto.Marshal(method)
	if err != nil {
		return nil, err
	}
	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_Buckets{
			Buckets: &iotexapi.ReadStakingDataRequest_VoteBuckets{
				Pagination: &iotexapi.PaginationParam{
					Offset: offset,
					Limit:  limit,
				},
			},
		},
	}
	argumentsBytes, err := proto.Marshal(arguments)
	if err != nil {
		return nil, err
	}
	return &CompositeBucketsStateContext{
		&protocol.BaseStateContext{
			Parameter: &protocol.Parameters{
				MethodName: methodBytes,
				Arguments:  [][]byte{argumentsBytes},
			},
		},
	}, nil
}

// EncodeToEth encode proto to eth
func (r *CompositeBucketsStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	var result iotextypes.VoteBucketList
	if err := proto.Unmarshal(resp.Data, &result); err != nil {
		return "", err
	}

	return stakingComm.EncodeVoteBucketListToEth(_compositeBucketsMethod.Outputs, &result)
}
