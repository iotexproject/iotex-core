// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package keypair

import (
	"strings"
	"testing"

	"github.com/iotexproject/iotex-core/address"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/stretchr/testify/require"
)

const (
	publicKey  = "04403d3c0dbd3270ddfc248c3df1f9aafd60f1d8e7456961c9ef26292262cc68f0ea9690263bef9e197a38f06026814fc70912c2b98d2e90a68f8ddc5328180a01"
	privateKey = "82a1556b2dbd0e3615e367edf5d3b90ce04346ec4d12ed71f67c70920ef9ac90"
)

func TestKeypair(t *testing.T) {
	require := require.New(t)

	_, err := DecodePublicKey("")
	require.True(strings.Contains(err.Error(), "invalid secp256k1 public key"))
	_, err = DecodePrivateKey("")
	require.True(strings.Contains(err.Error(), "invalid length, need 256 bits"))

	pubKey, err := DecodePublicKey(publicKey)
	require.NoError(err)
	priKey, err := DecodePrivateKey(privateKey)
	require.NoError(err)

	require.Equal(publicKey, EncodePublicKey(pubKey))
	require.Equal(privateKey, EncodePrivateKey(priKey))

	pubKeyBytes := PublicKeyToBytes(pubKey)
	priKeyBytes := PrivateKeyToBytes(priKey)

	_, err = BytesToPublicKey([]byte{1, 2, 3})
	require.Error(err)
	_, err = BytesToPrivateKey([]byte{4, 5, 6})
	require.Error(err)

	pk, err := BytesToPublicKey(pubKeyBytes)
	require.NoError(err)
	sk, err := BytesToPrivateKey(priKeyBytes)
	require.NoError(err)

	require.Equal(publicKey, EncodePublicKey(pk))
	require.Equal(privateKey, EncodePrivateKey(sk))

	_, err = StringToPubKeyBytes("")
	require.Error(err)

	_, err = StringToPubKeyBytes(publicKey)
	require.NoError(err)
}

func TestCompatibility(t *testing.T) {
	sk, err := crypto.GenerateKey()
	require.NoError(t, err)
	ethAddr := crypto.PubkeyToAddress(sk.PublicKey)
	pkHash := HashPubKey(&sk.PublicKey)
	addr, err := address.FromBytes(pkHash[:])
	require.NoError(t, err)
	require.Equal(t, ethAddr.Bytes(), addr.Bytes())
}
