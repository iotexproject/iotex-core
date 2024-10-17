package v3

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/abiutil"
	stakingComm "github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/common"
)

const _candidateByIDInterfaceABI = `[
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "id",
				"type": "address"
			}
		],
		"name": "candidateByIDV3",
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
					},
					{
						"internalType": "address",
						"name": "id",
						"type": "address"
					}
				],
				"internalType": "struct IStaking.CandidateV3",
				"name": "",
				"type": "tuple"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

var _candidateByIDMethod abi.Method

func init() {
	_candidateByIDMethod = abiutil.MustLoadMethod(_candidateByIDInterfaceABI, "candidateByIDV3")
}

func newCandidateByIDStateContext(data []byte) (*stakingComm.CandidateByAddressStateContext, error) {
	return NewCandidateByIDStateContext(data, &_candidateByIDMethod, iotexapi.ReadStakingDataMethod_CANDIDATE_BY_ADDRESS)
}

func NewCandidateByIDStateContext(data []byte, methodABI *abi.Method, apiMethod iotexapi.ReadStakingDataMethod_Name) (*stakingComm.CandidateByAddressStateContext, error) {
	paramsMap := map[string]interface{}{}
	ok := false
	if err := methodABI.Inputs.UnpackIntoMap(paramsMap, data); err != nil {
		return nil, err
	}
	var idAddress common.Address
	if idAddress, ok = paramsMap["id"].(common.Address); !ok {
		return nil, stakingComm.ErrDecodeFailure
	}
	id, err := address.FromBytes(idAddress[:])
	if err != nil {
		return nil, err
	}

	method := &iotexapi.ReadStakingDataMethod{
		Method: apiMethod,
	}
	methodBytes, err := proto.Marshal(method)
	if err != nil {
		return nil, err
	}
	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_CandidateByAddress_{
			CandidateByAddress: &iotexapi.ReadStakingDataRequest_CandidateByAddress{
				Id: id.String(),
			},
		},
	}
	argumentsBytes, err := proto.Marshal(arguments)
	if err != nil {
		return nil, err
	}
	return &stakingComm.CandidateByAddressStateContext{
		BaseStateContext: &protocol.BaseStateContext{
			Parameter: &protocol.Parameters{
				MethodName: methodBytes,
				Arguments:  [][]byte{argumentsBytes},
			},
			Method: methodABI,
		},
	}, nil
}
