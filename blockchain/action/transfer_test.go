// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/iotxaddress"
)

var chainid = []byte{0x00, 0x00, 0x00, 0x01}

func TestTransferSignVerify(t *testing.T) {
	assert := assert.New(t)
	sender, err := iotxaddress.NewAddress(true, chainid)
	assert.Nil(err)
	recipient, err := iotxaddress.NewAddress(true, chainid)
	assert.Nil(err)

	tsf := NewTransfer(0, big.NewInt(10), sender.RawAddress, recipient.RawAddress)
	assert.Nil(tsf.Signature)
	assert.NotNil(tsf.Verify(sender))

	// sign the transfer
	stsf, err := tsf.Sign(sender)
	assert.Nil(err)
	assert.NotNil(stsf)
	assert.Equal(tsf.Hash(), stsf.Hash())

	tsf.SenderPublicKey = sender.PublicKey
	assert.Equal(tsf.Hash(), stsf.Hash())

	// verify signature
	assert.Nil(stsf.Verify(sender))
	assert.NotNil(stsf.Verify(recipient))
	assert.NotNil(tsf.Signature)
	assert.Nil(tsf.Verify(sender))
}

func TestTransferSerializeDeserialize(t *testing.T) {
	assert := assert.New(t)
	sender, err := iotxaddress.NewAddress(true, chainid)
	assert.Nil(err)
	recipient, err := iotxaddress.NewAddress(true, chainid)
	assert.Nil(err)

	tsf := NewTransfer(0, big.NewInt(38291), sender.RawAddress, recipient.RawAddress)
	assert.NotNil(tsf)

	s, err := tsf.Serialize()
	assert.Nil(err)

	newtsf := &Transfer{}
	err = newtsf.Deserialize(s)
	assert.Nil(err)

	assert.Equal(uint64(0), newtsf.Nonce)
	assert.Equal(uint64(38291), newtsf.Amount.Uint64())
	assert.Equal(sender.RawAddress, newtsf.Sender)
	assert.Equal(recipient.RawAddress, newtsf.Recipient)

	assert.Equal(tsf.Hash(), newtsf.Hash())
	assert.Equal(tsf.TotalSize(), newtsf.TotalSize())
}
