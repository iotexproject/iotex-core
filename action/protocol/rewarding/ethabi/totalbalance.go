package ethabi

import (
	"encoding/hex"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/protocol"

	"github.com/iotexproject/iotex-core/action/protocol/abiutil"
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
	_totalBalanceMethod = abiutil.MustLoadMethod(_totalBalanceInterfaceABI, "totalBalance")
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
