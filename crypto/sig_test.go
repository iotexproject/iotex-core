// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package crypto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignVerify(t *testing.T) {
	require := require.New(t)
	pub, pri, err := NewKeyPair()
	require.NoError(err)

	message := []byte("hello iotex message")
	sig := Sign(pri, message)
	require.True(Verify(pub, message, sig))

	wrongMessage := []byte("wrong message")
	require.False(Verify(pub, wrongMessage, sig))
}

func TestPubKeyGeneration(t *testing.T) {
	require := require.New(t)
	expectedPuk, pri, err := NewKeyPair()
	require.NoError(err)

	actualPuk, err := NewPubKey(pri)
	require.NoError(err)
	require.Equal(expectedPuk, actualPuk)
	message := []byte("hello iotex message")
	sig := Sign(pri, message)
	require.True(Verify(actualPuk, message, sig))

	wrongMessage := []byte("wrong message")
	require.False(Verify(actualPuk, wrongMessage, sig))
}
