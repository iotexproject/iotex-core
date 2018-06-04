// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"testing"

	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/stretchr/testify/assert"
)

func TestVote(t *testing.T) {
	assert := assert.New(t)
	sender, err := iotxaddress.NewAddress(true, chainid)
	assert.Nil(err)
	recipient, err := iotxaddress.NewAddress(true, chainid)
	assert.Nil(err)
	// Create new vote and put on wire
	vote := NewVote(0, sender.PublicKey, recipient.PublicKey)
	raw, err := vote.Serialize()
	assert.Nil(err)
	// Sign the transfer
	signed, err := SignVote(raw, sender)
	assert.Nil(err)
	// deserialize
	vote2 := &Vote{}
	assert.Nil(vote2.Deserialize(signed))
	// test data match before/after serialization
	assert.Equal(vote.Hash(), vote2.Hash())
	// test signature
	assert.Nil(vote2.Verify(sender))
	assert.NotNil(vote2.Verify(recipient))
	assert.NotNil(vote.Verify(sender))
}
