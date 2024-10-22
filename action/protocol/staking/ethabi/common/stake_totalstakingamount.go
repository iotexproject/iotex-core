package common

import (
	"encoding/hex"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

// TotalStakingAmountStateContext context for TotalStakingAmount
type TotalStakingAmountStateContext struct {
	*protocol.BaseStateContext
}

func NewTotalStakingAmountContext(data []byte, methodABI *abi.Method, apiMethod iotexapi.ReadStakingDataMethod_Name) (*TotalStakingAmountStateContext, error) {
	method := &iotexapi.ReadStakingDataMethod{
		Method: apiMethod,
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
		&protocol.BaseStateContext{
			Parameter: &protocol.Parameters{
				MethodName: methodBytes,
				Arguments:  [][]byte{argumentsBytes},
			},
			Method: methodABI,
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
		return "", ErrConvertBigNumber
	}

	data, err := r.Method.Outputs.Pack(total)
	if err != nil {
		return "", nil
	}
	return hex.EncodeToString(data), nil
}
