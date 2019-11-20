package block

import (
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/stretchr/testify/require"
)

func TestBody_CalculateTxRoot(t *testing.T) {

	requireT := require.New(t)
	var sevlps []action.SealedEnvelope

	for i := 1; i <= 10; i++ {
		tsf, _ := action.NewTransfer(
			uint64(i),
			unit.ConvertIotxToRau(1000+int64(i)),
			identityset.Address(i%identityset.Size()).String(),
			nil,
			20000+uint64(i),
			unit.ConvertIotxToRau(1+int64(i)),
		)
		eb := action.EnvelopeBuilder{}
		evlp := eb.
			SetAction(tsf).
			SetGasLimit(tsf.GasLimit()).
			SetGasPrice(tsf.GasPrice()).
			SetNonce(tsf.Nonce()).
			SetVersion(1).
			Build()
		sevlp, err := action.Sign(evlp, identityset.PrivateKey((i+1)%identityset.Size()))
		requireT.NoError(err)
		sevlps = append(sevlps, sevlp)
	}

	c := calculateTxRoot(sevlps)

	c2 := []byte{30, 126, 187, 157, 243, 246, 95, 217, 142, 15, 248, 153, 223, 82, 169, 202, 94, 102, 14, 126,
		34, 232, 30, 47, 67, 118, 154, 16, 226, 232, 133, 197}
	c3 := hash.BytesToHash256(c2)
	requireT.Equal(c, c3)
}

func TestBody_CalculateTransferAmount(t *testing.T) {
	requireT := require.New(t)
	var sevlps []action.SealedEnvelope
	transferAmount := big.NewInt(0)

	for i := 1; i <= 10; i++ {
		tsf, _ := action.NewTransfer(
			uint64(i),
			unit.ConvertIotxToRau(1000+int64(i)),
			identityset.Address(i%identityset.Size()).String(),
			nil,
			20000+uint64(i),
			unit.ConvertIotxToRau(1+int64(i)),
		)
		eb := action.EnvelopeBuilder{}
		evlp := eb.
			SetAction(tsf).
			SetGasLimit(tsf.GasLimit()).
			SetGasPrice(tsf.GasPrice()).
			SetNonce(tsf.Nonce()).
			SetVersion(1).
			Build()
		sevlp, err := action.Sign(evlp, identityset.PrivateKey((i+1)%identityset.Size()))
		requireT.NoError(err)
		transferAmount.Add(transferAmount, tsf.Amount())
		sevlps = append(sevlps, sevlp)
	}

	amount := calculateTransferAmount(sevlps)
	requireT.Equal(amount, transferAmount)
}
