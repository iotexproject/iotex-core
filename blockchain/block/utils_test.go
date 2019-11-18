package block

import (
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/stretchr/testify/require"
)

func TestBody_CalculateTxRoot(t *testing.T) {

	var sevlps []action.SealedEnvelope
	h := make([]hash.Hash256, 0, 10)

	for i := 1; i <= 10; i++ {
		//i := rand.Int()
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
		sevlp, _ := action.Sign(evlp, identityset.PrivateKey((i+1)%identityset.Size()))
		h = append(h, sevlp.Hash())
		sevlps = append(sevlps, sevlp)
	}

	c := calculateTxRoot(sevlps)
	c2 := crypto.NewMerkleTree(h).HashTree()
	require.Equal(t, c, c2)
}

func TestBody_CalculateTransferAmount(t *testing.T) {

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
		sevlp, _ := action.Sign(evlp, identityset.PrivateKey((i+1)%identityset.Size()))
		transfer, _ := sevlp.Action().(*action.Transfer)
		transferAmount.Add(transferAmount, transfer.Amount())
		sevlps = append(sevlps, sevlp)
	}

	amount := calculateTransferAmount(sevlps)
	require.Equal(t, amount, transferAmount)
}
