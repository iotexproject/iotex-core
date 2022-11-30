package ethabi

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/stretchr/testify/require"
)

func TestUnclaimedBalanceEncodeToEth(t *testing.T) {
	r := require.New(t)

	ctx := &UnclaimedBalanceStateContext{}

	amount := big.NewInt(10000)
	resp := &iotexapi.ReadStateResponse{
		Data: []byte(amount.String()),
	}

	data, err := ctx.EncodeToEth(resp)
	r.Nil(err)
	r.EqualValues("0000000000000000000000000000000000000000000000000000000000002710", data)
}

func TestNewUnclaimedBalanceStateContext(t *testing.T) {
	r := require.New(t)

	data, _ := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000001")

	ctx, err := newUnclaimedBalanceStateContext(data)
	r.Nil(err)
	r.EqualValues("io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqps833xv", string(ctx.parameters.Arguments[0]))
	r.EqualValues("UnclaimedBalance", string(ctx.parameters.MethodName))
}
