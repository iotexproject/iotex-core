package action

import (
	"encoding/hex"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"
)

const _candidateByNameInterfaceABI = `[
	{
		"inputs": [
			{
				"internalType": "string",
				"name": "candName",
				"type": "string"
			}
		],
		"name": "candidateByName",
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

var _candidateByNameMethod abi.Method

func init() {
	_interface, err := abi.JSON(strings.NewReader(_candidateByNameInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	_candidateByNameMethod, ok = _interface.Methods["candidateByName"]
	if !ok {
		panic("fail to load the method")
	}
}

// CandidateByNameStateContext context for CandidateByName
type CandidateByNameStateContext struct {
	*baseStateContext
}

func newCandidateByNameStateContext(data []byte) (*CandidateByNameStateContext, error) {
	paramsMap := map[string]interface{}{}
	ok := false
	if err := _candidateByNameMethod.Inputs.UnpackIntoMap(paramsMap, data); err != nil {
		return nil, err
	}
	var candName string
	if candName, ok = paramsMap["candName"].(string); !ok {
		return nil, errDecodeFailure
	}

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_CANDIDATE_BY_NAME,
	}
	methodBytes, err := proto.Marshal(method)
	if err != nil {
		return nil, err
	}
	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_CandidateByName_{
			CandidateByName: &iotexapi.ReadStakingDataRequest_CandidateByName{
				CandName: candName,
			},
		},
	}
	argumentsBytes, err := proto.Marshal(arguments)
	if err != nil {
		return nil, err
	}
	return &CandidateByNameStateContext{
		&baseStateContext{
			&Parameters{
				MethodName: methodBytes,
				Arguments:  [][]byte{argumentsBytes},
			},
		},
	}, nil
}

// EncodeToEth encode proto to eth
func (r *CandidateByNameStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	var result iotextypes.CandidateV2
	if err := proto.Unmarshal(resp.Data, &result); err != nil {
		return "", err
	}

	cand, err := encodeCandidateToEth(&result)
	if err != nil {
		return "", err
	}

	data, err := _candidateByNameMethod.Outputs.Pack(cand)
	if err != nil {
		return "", nil
	}
	return hex.EncodeToString(data), nil
}
