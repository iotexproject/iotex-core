package staking

import (
	"encoding/hex"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"
)

const _candidatesInterfaceABI = `[
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
		"name": "candidates",
		"outputs": [
			{
				"components": [
					{
						"internalType": "address",
						"name": "ownerAddress",
						"type": "address"
					},
					{
						"internalType": "address",
						"name": "operatorAddress",
						"type": "address"
					},
					{
						"internalType": "address",
						"name": "rewardAddress",
						"type": "address"
					},
					{
						"internalType": "string",
						"name": "name",
						"type": "string"
					},
					{
						"internalType": "uint256",
						"name": "totalWeightedVotes",
						"type": "uint256"
					},
					{
						"internalType": "uint64",
						"name": "selfStakeBucketIdx",
						"type": "uint64"
					},
					{
						"internalType": "uint256",
						"name": "selfStakingTokens",
						"type": "uint256"
					}
				],
				"internalType": "struct IStaking.Candidate[]",
				"name": "",
				"type": "tuple[]"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

var _candidatesMethod abi.Method

func init() {
	_interface, err := abi.JSON(strings.NewReader(_candidatesInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	_candidatesMethod, ok = _interface.Methods["candidates"]
	if !ok {
		panic("fail to load the method")
	}
}

// CandidatesStateContext context for Candidates
type CandidatesStateContext struct {
	*baseStateContext
}

func newCandidatesStateContext(data []byte) (*CandidatesStateContext, error) {
	paramsMap := map[string]interface{}{}
	ok := false
	if err := _candidatesMethod.Inputs.UnpackIntoMap(paramsMap, data); err != nil {
		return nil, err
	}
	var offset, limit uint32
	if offset, ok = paramsMap["offset"].(uint32); !ok {
		return nil, errDecodeFailure
	}
	if limit, ok = paramsMap["limit"].(uint32); !ok {
		return nil, errDecodeFailure
	}

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_CANDIDATES,
	}
	methodBytes, err := proto.Marshal(method)
	if err != nil {
		return nil, err
	}
	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_Candidates_{
			Candidates: &iotexapi.ReadStakingDataRequest_Candidates{
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
	return &CandidatesStateContext{
		&baseStateContext{
			&Parameters{
				MethodName: methodBytes,
				Arguments:  [][]byte{argumentsBytes},
			},
		},
	}, nil
}

// EncodeToEth encode proto to eth
func (r *CandidatesStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	var result iotextypes.CandidateListV2
	if err := proto.Unmarshal(resp.Data, &result); err != nil {
		return "", err
	}

	args := make([]CandidateEth, len(result.Candidates))
	for i, candidate := range result.Candidates {
		cand, err := encodeCandidateToEth(candidate)
		if err != nil {
			return "", err
		}
		args[i] = *cand
	}

	data, err := _candidatesMethod.Outputs.Pack(args)
	if err != nil {
		return "", nil
	}
	return hex.EncodeToString(data), nil
}
