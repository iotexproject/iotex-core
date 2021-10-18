package block

import (
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"

	"github.com/iotexproject/go-pkgs/hash"
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

	c, err := calculateTxRoot(sevlps)
	require.NoError(t, err)

	c2 := []byte{158, 73, 244, 188, 155, 10, 251, 87, 98, 163, 234, 194, 38, 174,
		215, 255, 8, 148, 44, 204, 10, 56, 102, 180, 99, 188, 79, 146, 66, 219, 41, 30}
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

func TestVerifyBlock(t *testing.T) {
	require := require.New(t)

	tsf1, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), 1, big.NewInt(20), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	tsf2, err := action.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 1, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	blkhash, err := tsf1.Hash()
	require.NoError(err)
	blk, err := NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf1, tsf2).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(err)
	t.Run("success", func(t *testing.T) {
		require.NoError(VerifyBlock(&blk))
	})

	t.Run("wrong root hash", func(t *testing.T) {
		blk.Actions[0], blk.Actions[1] = blk.Actions[1], blk.Actions[0]
		require.Error(VerifyBlock(&blk))
	})
}
