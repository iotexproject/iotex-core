package staking

import (
	"encoding/hex"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"
)

const _candidateByAddressInterfaceABI = `[
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "ownerAddress",
				"type": "address"
			}
		],
		"name": "candidateByAddress",
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
				"internalType": "struct IStaking.Candidate",
				"name": "",
				"type": "tuple"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

var _candidateByAddressMethod abi.Method

func init() {
	_interface, err := abi.JSON(strings.NewReader(_candidateByAddressInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	_candidateByAddressMethod, ok = _interface.Methods["candidateByAddress"]
	if !ok {
		panic("fail to load the method")
	}
}

// CandidateByAddressStateContext context for candidateByAddress
type CandidateByAddressStateContext struct {
	*baseStateContext
}

func newCandidateByAddressStateContext(data []byte) (*CandidateByAddressStateContext, error) {
	paramsMap := map[string]interface{}{}
	ok := false
	if err := _candidateByAddressMethod.Inputs.UnpackIntoMap(paramsMap, data); err != nil {
		return nil, err
	}
	var ownerAddress common.Address
	if ownerAddress, ok = paramsMap["ownerAddress"].(common.Address); !ok {
		return nil, errDecodeFailure
	}
	owner, err := address.FromBytes(ownerAddress[:])
	if err != nil {
		return nil, err
	}

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_CANDIDATE_BY_ADDRESS,
	}
	methodBytes, err := proto.Marshal(method)
	if err != nil {
		return nil, err
	}
	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_CandidateByAddress_{
			CandidateByAddress: &iotexapi.ReadStakingDataRequest_CandidateByAddress{
				OwnerAddr: owner.String(),
			},
		},
	}
	argumentsBytes, err := proto.Marshal(arguments)
	if err != nil {
		return nil, err
	}
	return &CandidateByAddressStateContext{
		&baseStateContext{
			&Parameters{
				MethodName: methodBytes,
				Arguments:  [][]byte{argumentsBytes},
			},
		},
	}, nil
}

// EncodeToEth encode proto to eth
func (r *CandidateByAddressStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	var result iotextypes.CandidateV2
	if err := proto.Unmarshal(resp.Data, &result); err != nil {
		return "", err
	}

	cand, err := encodeCandidateToEth(&result)
	if err != nil {
		return "", err
	}

	data, err := _candidateByAddressMethod.Outputs.Pack(cand)
	if err != nil {
		return "", nil
	}
	return hex.EncodeToString(data), nil
}
