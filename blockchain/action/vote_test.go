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

func TestVoteSignVerify(t *testing.T) {
	assert := assert.New(t)
	sender, err := iotxaddress.NewAddress(true, chainid)
	assert.Nil(err)
	recipient, err := iotxaddress.NewAddress(true, chainid)
	assert.Nil(err)
	v := NewVote(0, sender.PublicKey, recipient.PublicKey)

	signedv, err := v.Sign(sender)
	assert.Nil(err)
	assert.Nil(signedv.Verify(sender))
	assert.NotNil(signedv.Verify(recipient))
}

func TestVoteSerializedDeserialize(t *testing.T) {
	assert := assert.New(t)
	sender, err := iotxaddress.NewAddress(true, chainid)
	assert.Nil(err)
	recipient, err := iotxaddress.NewAddress(true, chainid)
	assert.Nil(err)

	v := NewVote(0, sender.PublicKey, recipient.PublicKey)
	raw, err := v.Serialize()
	assert.Nil(err)

	newv := &Vote{}
	assert.Nil(newv.Deserialize(raw))
	assert.Equal(v.Hash(), newv.Hash())
	assert.Equal(v.TotalSize(), newv.TotalSize())
}
