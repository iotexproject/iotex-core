package ethabi

import (
	"encoding/hex"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	proto "github.com/iotexproject/iotex-proto/golang/protocol"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/abiutil"
)

const _unclaimedBalanceInterfaceABI = `[
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "account",
				"type": "address"
			}
		],
		"name": "unclaimedBalance",
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

var _unclaimedBalanceMethod abi.Method

func init() {
	_unclaimedBalanceMethod = abiutil.MustLoadMethod(_unclaimedBalanceInterfaceABI, "unclaimedBalance")
}

// UnclaimedBalanceStateContext context for UnclaimedBalance
type UnclaimedBalanceStateContext struct {
	*protocol.BaseStateContext
}

func newUnclaimedBalanceStateContext(data []byte) (*UnclaimedBalanceStateContext, error) {
	paramsMap := map[string]interface{}{}
	ok := false
	if err := _unclaimedBalanceMethod.Inputs.UnpackIntoMap(paramsMap, data); err != nil {
		return nil, err
	}
	var account common.Address
	if account, ok = paramsMap["account"].(common.Address); !ok {
		return nil, errDecodeFailure
	}
	accountAddress, err := address.FromBytes(account[:])
	if err != nil {
		return nil, err
	}

	return &UnclaimedBalanceStateContext{
		&protocol.BaseStateContext{
			Parameter: &protocol.Parameters{
				MethodName: []byte(proto.ReadUnclaimedBalanceMethodName),
				Arguments:  [][]byte{[]byte(accountAddress.String())},
			},
		},
	}, nil
}

// EncodeToEth encode proto to eth
func (r *UnclaimedBalanceStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	total, ok := new(big.Int).SetString(string(resp.Data), 10)
	if !ok {
		return "", errConvertBigNumber
	}

	data, err := _unclaimedBalanceMethod.Outputs.Pack(total)
	if err != nil {
		return "", nil
	}

	return hex.EncodeToString(data), nil
}
