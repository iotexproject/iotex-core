package ethabi

import (
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/protocol"
)

const _availableBalanceInterfaceABI = `[
	{
		"inputs": [],
		"name": "availableBalance",
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

var _availableBalanceMethod abi.Method

func init() {
	_interface, err := abi.JSON(strings.NewReader(_availableBalanceInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	_availableBalanceMethod, ok = _interface.Methods["availableBalance"]
	if !ok {
		panic("fail to load the method")
	}
}

// AvailableBalanceStateContext context for AvailableBalance
type AvailableBalanceStateContext struct {
	*baseStateContext
}

func newAvailableBalanceStateContext() (*AvailableBalanceStateContext, error) {
	return &AvailableBalanceStateContext{
		&baseStateContext{
			&Parameters{
				MethodName: []byte(protocol.ReadAvailableBalanceMethodName),
				Arguments:  nil,
			},
		},
	}, nil
}

// EncodeToEth encode proto to eth
func (r *AvailableBalanceStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	total, ok := new(big.Int).SetString(string(resp.Data), 10)
	if !ok {
		return "", errConvertBigNumber
	}

	data, err := _availableBalanceMethod.Outputs.Pack(total)
	if err != nil {
		return "", nil
	}

	return hex.EncodeToString(data), nil
}
