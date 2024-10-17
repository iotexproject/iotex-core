package ethabi

import (
	"encoding/hex"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	proto "github.com/iotexproject/iotex-proto/golang/protocol"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/abiutil"
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
	_availableBalanceMethod = abiutil.MustLoadMethod(_availableBalanceInterfaceABI, "availableBalance")
}

// AvailableBalanceStateContext context for AvailableBalance
type AvailableBalanceStateContext struct {
	*protocol.BaseStateContext
}

func newAvailableBalanceStateContext() (*AvailableBalanceStateContext, error) {
	return &AvailableBalanceStateContext{
		&protocol.BaseStateContext{
			Parameter: &protocol.Parameters{
				MethodName: []byte(proto.ReadAvailableBalanceMethodName),
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
