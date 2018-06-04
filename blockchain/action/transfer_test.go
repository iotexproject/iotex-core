// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/stretchr/testify/assert"
)

var chainid = []byte{0x00, 0x00, 0x00, 0x01}

func TestTransfer(t *testing.T) {
	assert := assert.New(t)
	sender, err := iotxaddress.NewAddress(true, chainid)
	assert.Nil(err)
	recipient, err := iotxaddress.NewAddress(true, chainid)
	assert.Nil(err)
	// Create new transfer and put on wire
	tsf := NewTransfer(0, 0, big.NewInt(10), sender.RawAddress, recipient.RawAddress)
	raw, err := tsf.Serialize()
	assert.Nil(err)
	// Sign the transfer
	signed, err := Sign(raw, sender)
	assert.Nil(err)
	// deserialize
	tsf2 := &Transfer{}
	assert.Nil(tsf2.Deserialize(signed))
	assert.NotEqual(tsf.Hash(), tsf2.Hash())
	// test data match before/after serialization
	tsf.SenderPublicKey = sender.PublicKey
	assert.Equal(tsf.Hash(), tsf2.Hash())
	// test signature
	assert.Nil(tsf2.Verify(sender))
	assert.NotNil(tsf2.Verify(recipient))
	assert.NotNil(tsf.Verify(sender))
}
