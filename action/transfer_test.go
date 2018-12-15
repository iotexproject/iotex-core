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

	"github.com/iotexproject/iotex-core/iotxaddress"
)

var chainid = []byte{0x00, 0x00, 0x00, 0x01}

func TestTransferSignVerify(t *testing.T) {
	require := require.New(t)
	sender, err := iotxaddress.NewAddress(iotxaddress.IsTestnet, chainid)
	require.NoError(err)
	recipient, err := iotxaddress.NewAddress(iotxaddress.IsTestnet, chainid)
	require.NoError(err)

	tsf, err := NewTransfer(0, big.NewInt(10), sender.RawAddress, recipient.RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)

	bd := &EnvelopeBuilder{}
	elp := bd.SetDestinationAddress(recipient.RawAddress).
		SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).
		SetAction(tsf).Build()

	w := AssembleSealedEnvelope(elp, sender.RawAddress, sender.PublicKey, []byte("lol"))
	require.Error(Verify(w))

	// sign the transfer
	selp, err := Sign(elp, sender.RawAddress, sender.PrivateKey)
	require.NoError(err)
	require.NotNil(selp)

	// verify signature
	require.NoError(Verify(selp))
}

func TestCoinbaseTsf(t *testing.T) {
	require := require.New(t)
	recipient, err := iotxaddress.NewAddress(iotxaddress.IsTestnet, chainid)
	require.NoError(err)
	coinbaseTsf := NewCoinBaseTransfer(1, big.NewInt(int64(5)), recipient.RawAddress)
	require.NotNil(t, coinbaseTsf)
	require.True(coinbaseTsf.isCoinbase)
}
