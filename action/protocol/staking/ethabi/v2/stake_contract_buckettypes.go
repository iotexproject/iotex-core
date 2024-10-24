package v2

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/abiutil"
	stakingComm "github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/common"
)

const _contractBucketTypesInterfaceABI = `[
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "contractAddress",
				"type": "address"
			}
		],
		"name": "contractStakeBucketTypes",
		"outputs": [
			{
				"components": [
					{
						"internalType": "uint256",
						"name": "stakedAmount",
						"type": "uint256"
					},
					{
						"internalType": "uint32",
						"name": "stakedDuration",
						"type": "uint32"
					}
				],
				"internalType": "struct IStaking.BucketType[]",
				"name": "",
				"type": "tuple[]"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

var _contractBucketTypesMethod abi.Method

func init() {
	_contractBucketTypesMethod = abiutil.MustLoadMethod(_contractBucketTypesInterfaceABI, "contractStakeBucketTypes")
}

// ContractBucketTypesStateContext context for Buckets
type ContractBucketTypesStateContext struct {
	*protocol.BaseStateContext
}

func newContractBucketTypesStateContext(data []byte) (*ContractBucketTypesStateContext, error) {
	paramsMap := map[string]interface{}{}
	ok := false
	if err := _contractBucketTypesMethod.Inputs.UnpackIntoMap(paramsMap, data); err != nil {
		return nil, err
	}
	var contractAddr common.Address
	if contractAddr, ok = paramsMap["contractAddress"].(common.Address); !ok {
		return nil, stakingComm.ErrDecodeFailure
	}
	contractAddress, err := address.FromBytes(contractAddr[:])
	if err != nil {
		return nil, err
	}

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_CONTRACT_STAKING_BUCKET_TYPES,
	}
	methodBytes, err := proto.Marshal(method)
	if err != nil {
		return nil, err
	}
	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_ContractStakingBucketTypes_{
			ContractStakingBucketTypes: &iotexapi.ReadStakingDataRequest_ContractStakingBucketTypes{
				ContractAddress: contractAddress.String(),
			},
		},
	}
	argumentsBytes, err := proto.Marshal(arguments)
	if err != nil {
		return nil, err
	}
	return &ContractBucketTypesStateContext{
		&protocol.BaseStateContext{
			Parameter: &protocol.Parameters{
				MethodName: methodBytes,
				Arguments:  [][]byte{argumentsBytes},
			},
		},
	}, nil
}

// EncodeToEth encode proto to eth
func (r *ContractBucketTypesStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	var result iotextypes.ContractStakingBucketTypeList
	if err := proto.Unmarshal(resp.Data, &result); err != nil {
		return "", err
	}

	return stakingComm.EncodeBucketTypeListToEth(_contractBucketTypesMethod.Outputs, &result)
}
