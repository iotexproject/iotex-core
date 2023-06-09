package ethabi

import (
	"encoding/hex"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/abiutil"
)

const _compositeTotalStakingAmountInterfaceABI = `[
	{
		"inputs": [],
		"name": "compositeTotalStakingAmount",
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

var _compositeTotalStakingAmountMethod abi.Method

func init() {
	_compositeTotalStakingAmountMethod = abiutil.MustLoadMethod(_compositeTotalStakingAmountInterfaceABI, "compositeTotalStakingAmount")
}

// CompositeTotalStakingAmountStateContext context for TotalStakingAmount
type CompositeTotalStakingAmountStateContext struct {
	*protocol.BaseStateContext
}

func newCompositeTotalStakingAmountContext() (*CompositeTotalStakingAmountStateContext, error) {
	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_COMPOSITE_TOTAL_STAKING_AMOUNT,
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
	return &CompositeTotalStakingAmountStateContext{
		&protocol.BaseStateContext{
			Parameter: &protocol.Parameters{
				MethodName: methodBytes,
				Arguments:  [][]byte{argumentsBytes},
			},
		},
	}, nil
}

// EncodeToEth encode proto to eth
func (r *CompositeTotalStakingAmountStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	var meta iotextypes.AccountMeta
	if err := proto.Unmarshal(resp.Data, &meta); err != nil {
		return "", err
	}

	total, ok := new(big.Int).SetString(meta.Balance, 10)
	if !ok {
		return "", errConvertBigNumber
	}

	data, err := _compositeTotalStakingAmountMethod.Outputs.Pack(total)
	if err != nil {
		return "", nil
	}
	return hex.EncodeToString(data), nil
}
