// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/testaddress"
)

var chainid = []byte{0x00, 0x00, 0x00, 0x01}

func TestTransferSignVerify(t *testing.T) {
	require := require.New(t)
	senderAddr := testaddress.Addrinfo["producer"]
	recipientAddr := testaddress.Addrinfo["alfa"]
	senderKey := testaddress.Keyinfo["producer"]

	tsf, err := NewTransfer(0, big.NewInt(10), senderAddr.Bech32(), recipientAddr.Bech32(), []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)

	bd := &EnvelopeBuilder{}
	elp := bd.SetDestinationAddress(recipientAddr.Bech32()).
		SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).
		SetAction(tsf).Build()

	w := AssembleSealedEnvelope(elp, senderAddr.Bech32(), senderKey.PubKey, []byte("lol"))
	require.Error(Verify(w))

	// sign the transfer
	selp, err := Sign(elp, senderAddr.Bech32(), senderKey.PriKey)
	require.NoError(err)
	require.NotNil(selp)

	// verify signature
	require.NoError(Verify(selp))
}

func TestCoinbaseTsf(t *testing.T) {
	require := require.New(t)
	recipientAddr := testaddress.Addrinfo["bravo"]
	coinbaseTsf := NewCoinBaseTransfer(1, big.NewInt(int64(5)), recipientAddr.Bech32())
	require.NotNil(t, coinbaseTsf)
	require.True(coinbaseTsf.isCoinbase)
}
