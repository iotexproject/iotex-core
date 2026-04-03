package action

import (
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func signedTransfer(t *testing.T, nonce uint64) *SealedEnvelope {
	t.Helper()
	se, err := SignedTransfer(
		identityset.Address(1).String(),
		identityset.PrivateKey(2),
		nonce,
		big.NewInt(1),
		nil,
		21000,
		big.NewInt(1),
	)
	require.NoError(t, err)
	return se
}

func signedClaim(t *testing.T) *SealedEnvelope {
	t.Helper()
	act := NewClaimFromRewardingFund(big.NewInt(1), nil, nil)
	elp := (&EnvelopeBuilder{}).SetNonce(0).SetGasLimit(ClaimFromRewardingFundBaseGas).SetGasPrice(big.NewInt(1)).SetAction(act).Build()
	selp, err := Sign(elp, identityset.PrivateKey(0))
	require.NoError(t, err)
	return selp
}

func TestBundle_ValidateItem(t *testing.T) {
	t.Run("Transfer allowed", func(t *testing.T) {
		b := NewBundle()
		require.NoError(t, b.Add(signedTransfer(t, 0)))
	})
	t.Run("Execution allowed", func(t *testing.T) {
		se, err := SignedExecution("", identityset.PrivateKey(2), 0, big.NewInt(0), 100000, big.NewInt(1), nil)
		require.NoError(t, err)
		b := NewBundle()
		require.NoError(t, b.Add(se))
	})
	t.Run("ClaimFromRewardingFund rejected", func(t *testing.T) {
		b := NewBundle()
		require.ErrorIs(t, b.Add(signedClaim(t)), ErrInvalidAct)
	})
	t.Run("nil item rejected", func(t *testing.T) {
		b := NewBundle()
		require.ErrorIs(t, b.Add(nil), ErrNilAction)
	})
	t.Run("nil Envelope rejected without panic", func(t *testing.T) {
		b := NewBundle()
		require.ErrorIs(t, b.Add(&SealedEnvelope{}), ErrNilAction)
	})
}

func TestBundle_Hash(t *testing.T) {
	t.Run("nil bundle returns zero hash", func(t *testing.T) {
		var b *Bundle
		require.Equal(t, hash.ZeroHash256, b.Hash())
	})
	t.Run("empty bundle returns zero hash", func(t *testing.T) {
		require.Equal(t, hash.ZeroHash256, NewBundle().Hash())
	})
	t.Run("same contents produce same hash", func(t *testing.T) {
		se := signedTransfer(t, 0)
		b1, b2 := NewBundle(), NewBundle()
		require.NoError(t, b1.Add(se))
		require.NoError(t, b2.Add(se))
		b1.SetTargetBlockHeight(10)
		b2.SetTargetBlockHeight(10)
		require.Equal(t, b1.Hash(), b2.Hash())
	})
	t.Run("different target height produces different hash", func(t *testing.T) {
		se := signedTransfer(t, 0)
		b1, b2 := NewBundle(), NewBundle()
		require.NoError(t, b1.Add(se))
		require.NoError(t, b2.Add(se))
		b1.SetTargetBlockHeight(10)
		b2.SetTargetBlockHeight(20)
		require.NotEqual(t, b1.Hash(), b2.Hash())
	})
}

func TestBundle_Gas(t *testing.T) {
	t.Run("nil bundle returns 0", func(t *testing.T) {
		var b *Bundle
		require.Zero(t, b.Gas())
	})
	t.Run("empty bundle returns 0", func(t *testing.T) {
		require.Zero(t, NewBundle().Gas())
	})
	t.Run("sums gas of all items", func(t *testing.T) {
		se1 := signedTransfer(t, 0) // gas 21000
		se2 := signedTransfer(t, 1) // gas 21000
		b := NewBundle()
		require.NoError(t, b.Add(se1, se2))
		require.Equal(t, se1.Gas()+se2.Gas(), b.Gas())
	})
}

func TestBundle_ProtoRoundTrip(t *testing.T) {
	se1 := signedTransfer(t, 0)
	se2 := signedTransfer(t, 1)
	orig := NewBundle()
	require.NoError(t, orig.Add(se1, se2))
	orig.SetTargetBlockHeight(42)

	pb := orig.Proto()
	require.NotNil(t, pb)
	require.Equal(t, uint64(42), pb.TargetBlockNumber)
	require.Equal(t, 2, len(pb.Actions))

	restored := NewBundle()
	require.NoError(t, restored.LoadProto(pb, (&Deserializer{}).SetEvmNetworkID(1)))
	require.Equal(t, orig.Len(), restored.Len())
	require.Equal(t, orig.TargetBlockHeight(), restored.TargetBlockHeight())
	require.Equal(t, orig.Gas(), restored.Gas())
	require.Equal(t, orig.Hash(), restored.Hash())
}
