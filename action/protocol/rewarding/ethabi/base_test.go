package ethabi

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildReadStateRequest(t *testing.T) {
	r := require.New(t)

	data, _ := hex.DecodeString("1234")
	ctx, err := BuildReadStateRequest(data)
	r.Nil(ctx)
	r.EqualError(errInvalidCallData, err.Error())

	data, _ = hex.DecodeString("12345678")
	ctx, err = BuildReadStateRequest(data)
	r.Nil(ctx)
	r.EqualError(errInvalidCallSig, err.Error())

	data, _ = hex.DecodeString("ad7a672f")
	ctx, err = BuildReadStateRequest(data)
	r.Nil(err)
	r.IsType(&TotalBalanceStateContext{}, ctx)

	data, _ = hex.DecodeString("ab2f0e51")
	ctx, err = BuildReadStateRequest(data)
	r.Nil(err)
	r.IsType(&AvailableBalanceStateContext{}, ctx)

	data, _ = hex.DecodeString("01cbf5fb0000000000000000000000000000000000000000000000000000000000000001")
	ctx, err = BuildReadStateRequest(data)
	r.Nil(err)
	r.IsType(&UnclaimedBalanceStateContext{}, ctx)
}
