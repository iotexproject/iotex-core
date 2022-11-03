package action

import (
	"encoding/hex"
	"math/big"
	"reflect"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestCallDataToStakeStateContext_TotalStakingAmount(t *testing.T) {
	r := require.New(t)

	data, _ := hex.DecodeString("d201114a")
	req, err := CallDataToStakeStateContext(data)

	r.Nil(err)
	r.EqualValues("*action.TotalStakingAmountStateContext", reflect.TypeOf(req).String())

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_TOTAL_STAKING_AMOUNT,
	}
	methodBytes, _ := proto.Marshal(method)
	r.EqualValues(methodBytes, req.Parameters().MethodName)

	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_TotalStakingAmount_{
			TotalStakingAmount: &iotexapi.ReadStakingDataRequest_TotalStakingAmount{},
		},
	}
	argumentsBytes, _ := proto.Marshal(arguments)
	r.EqualValues([][]byte{argumentsBytes}, req.Parameters().Arguments)
}

func TestEncodeTotalStakingAmountToEth(t *testing.T) {
	r := require.New(t)

	total, _ := new(big.Int).SetString("100000000000000000000", 10)
	resp := &iotexapi.ReadStateResponse{
		Data: total.Bytes(),
	}

	ctx := &TotalStakingAmountStateContext{}
	data, err := ctx.EncodeToEth(resp)
	r.Nil(err)
	r.EqualValues("0000000000000000000000000000000000000000000000056bc75e2d63100000", data)
}
