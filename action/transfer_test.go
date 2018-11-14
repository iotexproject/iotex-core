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
	require.Nil(tsf.signature)
	require.Error(Verify(tsf))

	// sign the transfer
	require.NoError(Sign(tsf, sender.PrivateKey))

	// verify signature
	require.NoError(Verify(tsf))
}

func TestTransferSerializeDeserialize(t *testing.T) {
	require := require.New(t)
	sender, err := iotxaddress.NewAddress(iotxaddress.IsTestnet, chainid)
	require.NoError(err)
	recipient, err := iotxaddress.NewAddress(iotxaddress.IsTestnet, chainid)
	require.NoError(err)

	tsf, err := NewTransfer(0, big.NewInt(38291), sender.RawAddress, recipient.RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.NotNil(tsf)

	s, err := tsf.Serialize()
	require.NoError(err)

	newtsf := &Transfer{}
	err = newtsf.Deserialize(s)
	require.NoError(err)

	require.Equal(uint64(0), newtsf.Nonce())
	require.Equal(uint64(38291), newtsf.amount.Uint64())
	require.Equal(sender.RawAddress, newtsf.Sender())
	require.Equal(recipient.RawAddress, newtsf.Recipient())

	require.Equal(tsf.Hash(), newtsf.Hash())
	require.Equal(tsf.TotalSize(), newtsf.TotalSize())
}

func TestTransferToJSONFromJSON(t *testing.T) {
	require := require.New(t)
	sender, err := iotxaddress.NewAddress(iotxaddress.IsTestnet, chainid)
	require.NoError(err)
	recipient, err := iotxaddress.NewAddress(iotxaddress.IsTestnet, chainid)
	require.NoError(err)

	tsf, err := NewTransfer(0, big.NewInt(38291), sender.RawAddress, recipient.RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.NotNil(tsf)

	exptsf := tsf.ToJSON()
	require.NotNil(exptsf)

	newtsf, err := NewTransferFromJSON(exptsf)
	require.NoError(err)
	require.NotNil(newtsf)

	require.Equal(uint64(0), newtsf.Nonce())
	require.Equal(uint64(38291), newtsf.amount.Uint64())
	require.Equal(sender.RawAddress, newtsf.Sender())
	require.Equal(recipient.RawAddress, newtsf.Recipient())

	require.Equal(tsf.Hash(), newtsf.Hash())
	require.Equal(tsf.TotalSize(), newtsf.TotalSize())
}

func TestCoinbaseTsf(t *testing.T) {
	require := require.New(t)
	recipient, err := iotxaddress.NewAddress(iotxaddress.IsTestnet, chainid)
	require.NoError(err)
	coinbaseTsf := NewCoinBaseTransfer(1, big.NewInt(int64(5)), recipient.RawAddress)
	require.NotNil(t, coinbaseTsf)
	require.True(coinbaseTsf.isCoinbase)
}
