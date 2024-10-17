package block

import (
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"
)

func TestBody_CalculateTxRoot(t *testing.T) {
	requireT := require.New(t)
	var sevlps []*action.SealedEnvelope

	for i := 1; i <= 10; i++ {
		tsf := action.NewTransfer(
			unit.ConvertIotxToRau(1000+int64(i)),
			identityset.Address(i%identityset.Size()).String(),
			nil)
		eb := action.EnvelopeBuilder{}
		evlp := eb.SetNonce(uint64(i)).SetGasPrice(unit.ConvertIotxToRau(1 + int64(i))).
			SetGasLimit(20000 + uint64(i)).SetAction(tsf).Build()
		sevlp, err := action.Sign(evlp, identityset.PrivateKey((i+1)%identityset.Size()))
		requireT.NoError(err)
		sevlps = append(sevlps, sevlp)
	}

	c, err := calculateTxRoot(sevlps)
	require.NoError(t, err)

	c2 := []byte{158, 73, 244, 188, 155, 10, 251, 87, 98, 163, 234, 194, 38, 174,
		215, 255, 8, 148, 44, 204, 10, 56, 102, 180, 99, 188, 79, 146, 66, 219, 41, 30}
	c3 := hash.BytesToHash256(c2)
	requireT.Equal(c, c3)
}

func TestBody_CalculateTransferAmount(t *testing.T) {
	requireT := require.New(t)
	var sevlps []*action.SealedEnvelope
	transferAmount := big.NewInt(0)

	for i := 1; i <= 10; i++ {
		tsf := action.NewTransfer(
			unit.ConvertIotxToRau(1000+int64(i)),
			identityset.Address(i%identityset.Size()).String(),
			nil)
		eb := action.EnvelopeBuilder{}
		evlp := eb.SetNonce(uint64(i)).SetGasPrice(unit.ConvertIotxToRau(1 + int64(i))).
			SetGasLimit(20000 + uint64(i)).SetAction(tsf).Build()
		sevlp, err := action.Sign(evlp, identityset.PrivateKey((i+1)%identityset.Size()))
		requireT.NoError(err)
		transferAmount.Add(transferAmount, tsf.Amount())
		sevlps = append(sevlps, sevlp)
	}

	amount := calculateTransferAmount(sevlps)
	requireT.Equal(amount, transferAmount)
}
