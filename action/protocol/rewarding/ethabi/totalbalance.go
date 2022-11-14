package ethabi

import (
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/protocol"
)

const _totalBalanceInterfaceABI = `[
	{
		"inputs": [],
		"name": "totalBalance",
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

var _totalBalanceMethod abi.Method

func init() {
	_interface, err := abi.JSON(strings.NewReader(_totalBalanceInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	_totalBalanceMethod, ok = _interface.Methods["totalBalance"]
	if !ok {
		panic("fail to load the method")
	}
}

// TotalBalanceStateContext context for TotalBalance
type TotalBalanceStateContext struct {
	*baseStateContext
}

func newTotalBalanceStateContext() (*TotalBalanceStateContext, error) {
	return &TotalBalanceStateContext{
		&baseStateContext{
			&Parameters{
				MethodName: []byte(protocol.ReadTotalBalanceMethodName),
				Arguments:  nil,
			},
		},
	}, nil
}

// EncodeToEth encode proto to eth
func (r *TotalBalanceStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	total, ok := new(big.Int).SetString(string(resp.Data), 10)
	if !ok {
		return "", errConvertBigNumber
	}

	data, err := _totalBalanceMethod.Outputs.Pack(total)
	if err != nil {
		return "", nil
	}

	return hex.EncodeToString(data), nil
}
