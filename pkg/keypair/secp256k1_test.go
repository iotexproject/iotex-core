package keypair

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/pkg/hash"
)

func TestSecp256k1(t *testing.T) {
	require := require.New(t)

	sk, err := newSecp256k1PrvKey()
	require.NoError(err)
	defer sk.Zero()
	require.Equal(secp256prvKeyLength, len(sk.Bytes()))
	pk := sk.PublicKey()
	require.Equal(secp256pubKeyLength, len(pk.Bytes()))
	nsk, err := newSecp256k1PrvKeyFromBytes(sk.Bytes())
	require.NoError(err)
	require.Equal(sk, nsk)
	npk, err := newSecp256k1PubKeyFromBytes(pk.Bytes())
	require.NoError(err)
	require.Equal(pk, npk)

	h := hash.Hash256b([]byte("test secp256k1 signature så∫jaç∂fla´´3jl©˙kl3∆˚83jl≈¥fjs2"))
	sig, err := sk.Sign(h[:])
	require.NoError(err)
	require.True(sig[secp256pubKeyLength-1] == 0 || sig[secp256pubKeyLength-1] == 1)
	require.True(pk.Verify(h[:], sig))
	for i := 0; i < len(sig)-1; i++ {
		fsig := make([]byte, len(sig))
		copy(fsig, sig)
		fsig[i]--
		require.False(pk.Verify(h[:], fsig))
	}
	sig[secp256pubKeyLength-1] = 2
	require.False(pk.Verify(h[:], sig))
}
