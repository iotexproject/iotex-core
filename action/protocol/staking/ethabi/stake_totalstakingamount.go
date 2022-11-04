package ethabi

import (
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"
)

var _totalStakingAmountInterfaceABI = `[
	{
		"inputs": [],
		"name": "totalStakingAmount",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "",
				"type": "uint256"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

var _totalStakingAmountMethod abi.Method

func init() {
	_interface, err := abi.JSON(strings.NewReader(_totalStakingAmountInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	_totalStakingAmountMethod, ok = _interface.Methods["totalStakingAmount"]
	if !ok {
		panic("fail to load the method")
	}
}

// TotalStakingAmountStateContext context for TotalStakingAmount
type TotalStakingAmountStateContext struct {
	*baseStateContext
}

func newTotalStakingAmountContext() (*TotalStakingAmountStateContext, error) {
	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_TOTAL_STAKING_AMOUNT,
	}
	methodBytes, err := proto.Marshal(method)
	if err != nil {
		return nil, err
	}
	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_TotalStakingAmount_{
			TotalStakingAmount: &iotexapi.ReadStakingDataRequest_TotalStakingAmount{},
		},
	}
	argumentsBytes, err := proto.Marshal(arguments)
	if err != nil {
		return nil, err
	}
	return &TotalStakingAmountStateContext{
		&baseStateContext{
			&Parameters{
				MethodName: methodBytes,
				Arguments:  [][]byte{argumentsBytes},
			},
		},
	}, nil
}

// EncodeToEth encode proto to eth
func (r *TotalStakingAmountStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	var meta iotextypes.AccountMeta
	if err := proto.Unmarshal(resp.Data, &meta); err != nil {
		return "", err
	}

	total, ok := new(big.Int).SetString(meta.Balance, 10)
	if !ok {
		return "", errConvertBigNumber
	}

	data, err := _totalStakingAmountMethod.Outputs.Pack(total)
	if err != nil {
		return "", nil
	}
	return hex.EncodeToString(data), nil
}
