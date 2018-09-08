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
	ta "github.com/iotexproject/iotex-core/test/testaddress"
)

var chainid = []byte{0x00, 0x00, 0x00, 0x01}

func TestTransferSignVerify(t *testing.T) {
	require := require.New(t)
	sender, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)
	recipient, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)

	tsf, err := NewTransfer(0, big.NewInt(10), sender.RawAddress, recipient.RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.Nil(tsf.Signature)
	require.NotNil(tsf.Verify(sender))

	// sign the transfer
	stsf, err := tsf.Sign(sender)
	require.NoError(err)
	require.NotNil(stsf)
	require.Equal(tsf.Hash(), stsf.Hash())

	tsf.SenderPublicKey = sender.PublicKey
	require.Equal(tsf.Hash(), stsf.Hash())

	// verify signature
	require.NoError(stsf.Verify(sender))
	require.NotNil(stsf.Verify(recipient))
	require.NotNil(tsf.Signature)
	require.NoError(tsf.Verify(sender))
}

func TestTransferSerializeDeserialize(t *testing.T) {
	require := require.New(t)
	sender, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)
	recipient, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)

	tsf, err := NewTransfer(0, big.NewInt(38291), sender.RawAddress, recipient.RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.NotNil(tsf)

	s, err := tsf.Serialize()
	require.NoError(err)

	newtsf := &Transfer{}
	err = newtsf.Deserialize(s)
	require.NoError(err)

	require.Equal(uint64(0), newtsf.Nonce)
	require.Equal(uint64(38291), newtsf.Amount.Uint64())
	require.Equal(sender.RawAddress, newtsf.Sender)
	require.Equal(recipient.RawAddress, newtsf.Recipient)

	require.Equal(tsf.Hash(), newtsf.Hash())
	require.Equal(tsf.TotalSize(), newtsf.TotalSize())
}

func TestTransferToJSONFromJSON(t *testing.T) {
	require := require.New(t)
	sender, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)
	recipient, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)

	tsf, err := NewTransfer(0, big.NewInt(38291), sender.RawAddress, recipient.RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.NotNil(tsf)

	exptsf := tsf.ToJSON()
	require.NotNil(exptsf)

	newtsf, err := NewTransferFromJSON(exptsf)
	require.NoError(err)
	require.NotNil(newtsf)

	require.Equal(uint64(0), newtsf.Nonce)
	require.Equal(uint64(38291), newtsf.Amount.Uint64())
	require.Equal(sender.RawAddress, newtsf.Sender)
	require.Equal(recipient.RawAddress, newtsf.Recipient)

	require.Equal(tsf.Hash(), newtsf.Hash())
	require.Equal(tsf.TotalSize(), newtsf.TotalSize())
}

func TestCoinbaseTsf(t *testing.T) {
	coinbaseTsf := NewCoinBaseTransfer(big.NewInt(int64(5)), ta.Addrinfo["producer"].RawAddress)
	require.NotNil(t, coinbaseTsf)
	require.True(t, coinbaseTsf.IsCoinbase)
}
