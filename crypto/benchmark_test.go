package crypto

import (
	"testing"

	"github.com/iotexproject/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"
)

func BenchmarkEc283_Verify(b *testing.B) {
	for n := 0; n < b.N; n++ {
		VerifyEC283Signature(b)
	}
}

func BenchmarkSecp256_Verify(b *testing.B) {
	for n := 0; n < b.N; n++ {
		VerifySECP256Signature(b)
	}
}

func VerifyEC283Signature(b *testing.B) {
	require := require.New(b)

	pk, sk, _ := EC283.NewKeyPair()
	msg := blake2b.Sum256([]byte{1, 2, 3})
	sig := EC283.Sign(sk, msg[:])
	require.True(EC283.Verify(pk, msg[:], sig))
}

func VerifySECP256Signature(b *testing.B) {
	require := require.New(b)

	sk, _ := crypto.GenerateKey()
	pk := crypto.FromECDSAPub(&sk.PublicKey)
	msg := blake2b.Sum256([]byte{1, 2, 3})
	sig, err := crypto.Sign(msg[:], sk)
	require.NoError(err)
	require.True(crypto.VerifySignature(pk, msg[:], sig[:64]))
}
