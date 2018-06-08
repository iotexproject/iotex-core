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

func TestTransfer(t *testing.T) {
	assert := assert.New(t)
	sender, err := iotxaddress.NewAddress(true, chainid)
	assert.Nil(err)
	recipient, err := iotxaddress.NewAddress(true, chainid)
	assert.Nil(err)

	// Create a nil transfer and sign it
	var nilTsf *Transfer
	nilTsf = nil
	sNilTsf, err := SignTransfer(nilTsf, sender)
	assert.Nil(sNilTsf)
	assert.NotNil(err)

	// Create new transfer and put on wire
	tsf := NewTransfer(0, 0, big.NewInt(10), sender.RawAddress, recipient.RawAddress)
	assert.Nil(tsf.Signature)
	assert.NotNil(tsf.Verify(sender))

	// Sign the transfer
	stsf, err := SignTransfer(tsf, sender)
	assert.Nil(err)
	assert.NotNil(stsf)
	assert.Equal(tsf.Hash(), stsf.Hash())

	tsf.SenderPublicKey = sender.PublicKey
	assert.Equal(tsf.Hash(), stsf.Hash())

	// test signature
	assert.Nil(stsf.Verify(sender))
	assert.NotNil(stsf.Verify(recipient))
	assert.NotNil(tsf.Signature)
	assert.Nil(tsf.Verify(sender))
}
