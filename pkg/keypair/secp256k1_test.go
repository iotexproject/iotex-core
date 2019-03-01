package keypair

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/pkg/hash"
)

func TestSecp256k1(t *testing.T) {
	require := require.New(t)

	k1, err := NewSecp256k1PrvKey()
	require.NoError(err)
	k2, err := NewSecp256k1PrvKey()
	require.NoError(err)
	k3, err := NewSecp256k1PrvKey()
	require.NoError(err)
	k4, err := NewSecp256k1PrvKey()
	require.NoError(err)

	tests := []PrivateKey{k1, k2, k3, k4}

	for _, sk := range tests {
		require.Equal(secp256prvKeyLength, len(sk.PrvKeyBytes()))
		pk := sk.PubKey()
		require.Equal(secp256pubKeyLength, len(pk.PubKeyBytes()))
		nsk, err := NewSecp256k1PrvKeyFromBytes(sk.PrvKeyBytes())
		require.NoError(err)
		require.Equal(sk, nsk)
		npk, err := NewSecp256k1PubKeyFromBytes(pk.PubKeyBytes())
		require.NoError(err)
		require.Equal(pk, npk)

		h := hash.Hash256b([]byte("test secp256k1 signature så∫jaç∂fla´´3jl©˙kl3∆˚83jl≈¥fjs2"))
		sig, err := sk.Sign(h[:])
		require.NoError(err)
		require.True(sig[64] == 0 || sig[64] == 1)
		require.True(pk.Verify(h[:], sig))
		sig[3] = sig[3] - 1
		require.False(pk.Verify(h[:], sig))
	}
}
