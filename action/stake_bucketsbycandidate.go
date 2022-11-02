package action

import (
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"
)

const _bucketsByCandidateInterfaceABI = `[
	{
		"inputs": [
			{
				"internalType": "string",
				"name": "candName",
				"type": "string"
			},
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
		"name": "bucketsByCandidate",
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
					}
				],
				"internalType": "struct IStaking.VoteBucket[]",
				"name": "",
				"type": "tuple[]"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

var _bucketsByCandidateMethod abi.Method

func init() {
	_interface, err := abi.JSON(strings.NewReader(_bucketsByCandidateInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	_bucketsByCandidateMethod, ok = _interface.Methods["bucketsByCandidate"]
	if !ok {
		panic("fail to load the method")
	}
}

// BucketsByCandidateStateContext context for BucketsByCandidate
type BucketsByCandidateStateContext struct {
	*baseStateContext
}

func newBucketsByCandidateStateContext(data []byte) (*BucketsByCandidateStateContext, error) {
	paramsMap := map[string]interface{}{}
	ok := false
	if err := _bucketsByCandidateMethod.Inputs.UnpackIntoMap(paramsMap, data); err != nil {
		return nil, err
	}
	var candName string
	if candName, ok = paramsMap["candName"].(string); !ok {
		return nil, errDecodeFailure
	}
	var offset, limit uint32
	if offset, ok = paramsMap["offset"].(uint32); !ok {
		return nil, errDecodeFailure
	}
	if limit, ok = paramsMap["limit"].(uint32); !ok {
		return nil, errDecodeFailure
	}

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_BUCKETS_BY_CANDIDATE,
	}
	methodBytes, err := proto.Marshal(method)
	if err != nil {
		return nil, err
	}
	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketsByCandidate{
			BucketsByCandidate: &iotexapi.ReadStakingDataRequest_VoteBucketsByCandidate{
				CandName: candName,
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
	return &BucketsByCandidateStateContext{
		&baseStateContext{
			&Parameters{
				MethodName: methodBytes,
				Arguments:  [][]byte{argumentsBytes},
			},
		},
	}, nil
}

// EncodeToEth encode proto to eth
func (r *BucketsByCandidateStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	var result iotextypes.VoteBucketList
	if err := proto.Unmarshal(resp.Data, &result); err != nil {
		return "", err
	}

	return encodeVoteBucketListToEth(result)
}
