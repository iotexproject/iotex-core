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
	recipientAddr := testaddress.Addrinfo["alfa"]
	senderKey := testaddress.Keyinfo["producer"]

	tsf, err := NewTransfer(0, big.NewInt(10), recipientAddr.String(), []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)

	tsf.Proto()

	bd := &EnvelopeBuilder{}
	elp := bd.SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).
		SetAction(tsf).Build()

	elp.ByteStream()

	w := AssembleSealedEnvelope(elp, senderKey.PubKey, []byte("lol"))
	require.Error(Verify(w))

	// sign the transfer
	selp, err := Sign(elp, senderKey.PriKey)
	require.NoError(err)
	require.NotNil(selp)

	// verify signature
	require.NoError(Verify(selp))
}
