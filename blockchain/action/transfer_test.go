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
	sender, err := iotxaddress.NewAddress(true, chainid)
	require.Nil(err)
	recipient, err := iotxaddress.NewAddress(true, chainid)
	require.Nil(err)

	tsf := NewTransfer(0, big.NewInt(10), sender.RawAddress, recipient.RawAddress)
	require.Nil(tsf.Signature)
	require.NotNil(tsf.Verify(sender))

	// sign the transfer
	stsf, err := tsf.Sign(sender)
	require.Nil(err)
	require.NotNil(stsf)
	require.Equal(tsf.Hash(), stsf.Hash())

	tsf.SenderPublicKey = sender.PublicKey
	require.Equal(tsf.Hash(), stsf.Hash())

	// verify signature
	require.Nil(stsf.Verify(sender))
	require.NotNil(stsf.Verify(recipient))
	require.NotNil(tsf.Signature)
	require.Nil(tsf.Verify(sender))
}

func TestTransferSerializeDeserialize(t *testing.T) {
	require := require.New(t)
	sender, err := iotxaddress.NewAddress(true, chainid)
	require.Nil(err)
	recipient, err := iotxaddress.NewAddress(true, chainid)
	require.Nil(err)

	tsf := NewTransfer(0, big.NewInt(38291), sender.RawAddress, recipient.RawAddress)
	require.NotNil(tsf)

	s, err := tsf.Serialize()
	require.Nil(err)

	newtsf := &Transfer{}
	err = newtsf.Deserialize(s)
	require.Nil(err)

	require.Equal(uint64(0), newtsf.Nonce)
	require.Equal(uint64(38291), newtsf.Amount.Uint64())
	require.Equal(sender.RawAddress, newtsf.Sender)
	require.Equal(recipient.RawAddress, newtsf.Recipient)

	require.Equal(tsf.Hash(), newtsf.Hash())
	require.Equal(tsf.TotalSize(), newtsf.TotalSize())
}
