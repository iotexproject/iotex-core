// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package keypair

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

const (
	publicKey  = "336eb60a5741f585a8e81de64e071327a3b96c15af4af5723598a07b6121e8e813bbd0056ba71ae29c0d64252e913f60afaeb11059908b81ff27cbfa327fd371d35f5ec0cbc01705"
	privateKey = "925f0c9e4b6f6d92f2961d01aff6204c44d73c0b9d0da188582932d4fcad0d8ee8c66600"
)

func TestKeypair(t *testing.T) {
	require := require.New(t)

	_, err := DecodePublicKey("")
	require.Equal(ErrPublicKey, errors.Cause(err))
	_, err = DecodePrivateKey("")
	require.Equal(ErrPrivateKey, errors.Cause(err))

	pubKey, err := DecodePublicKey(publicKey)
	require.Nil(err)
	priKey, err := DecodePrivateKey(privateKey)
	require.Nil(err)

	require.Equal(publicKey, EncodePublicKey(pubKey))
	require.Equal(privateKey, EncodePrivateKey(priKey))

	_, err = StringToPubKeyBytes("")
	require.Equal(ErrPublicKey, errors.Cause(err))
	_, err = StringToPriKeyBytes("")
	require.Equal(ErrPrivateKey, errors.Cause(err))

	pubKeyBytes, err := StringToPubKeyBytes(publicKey)
	require.Nil(err)
	priKeyBytes, err := StringToPriKeyBytes(privateKey)
	require.Nil(err)

	pubKeyString, err := BytesToPubKeyString(pubKeyBytes)
	require.Nil(err)
	priKeyString, err := BytesToPriKeyString(priKeyBytes)
	require.Nil(err)
	require.Equal(publicKey, pubKeyString)
	require.Equal(privateKey, priKeyString)
}
