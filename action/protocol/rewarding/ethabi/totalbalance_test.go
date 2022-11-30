package ethabi

import (
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/stretchr/testify/require"
)

func TestTotalBalanceEncodeToEth(t *testing.T) {
	r := require.New(t)

	ctx, err := newTotalBalanceStateContext()
	r.Nil(err)
	r.EqualValues("TotalBalance", string(ctx.parameters.MethodName))

	amount := big.NewInt(10000)
	resp := &iotexapi.ReadStateResponse{
		Data: []byte(amount.String()),
	}

	data, err := ctx.EncodeToEth(resp)
	r.Nil(err)
	r.EqualValues("0000000000000000000000000000000000000000000000000000000000002710", data)
}
