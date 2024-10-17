package v2

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/common"
)

func TestBuildReadStateRequestCompositeTotalStakingAmount(t *testing.T) {
	r := require.New(t)

	data, _ := hex.DecodeString("3aad591c")
	req, err := BuildReadStateRequest(data)

	r.Nil(err)
	r.EqualValues("*common.TotalStakingAmountStateContext", reflect.TypeOf(req).String())

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_COMPOSITE_TOTAL_STAKING_AMOUNT,
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

func TestEncodeCompositeTotalStakingAmountToEth(t *testing.T) {
	r := require.New(t)

	meta := &iotextypes.AccountMeta{
		Address: "io000000000000000000000000stakingprotocol",
		Balance: "100000000000000000000",
	}
	metaBytes, _ := proto.Marshal(meta)
	resp := &iotexapi.ReadStateResponse{
		Data: metaBytes,
	}

	ctx := &common.TotalStakingAmountStateContext{
		BaseStateContext: &protocol.BaseStateContext{
			Method: &_compositeTotalStakingAmountMethod,
		},
	}
	data, err := ctx.EncodeToEth(resp)
	r.Nil(err)
	r.EqualValues("0000000000000000000000000000000000000000000000056bc75e2d63100000", data)
}
