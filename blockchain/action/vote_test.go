// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/iotxaddress"
)

func TestVoteSignVerify(t *testing.T) {
	require := require.New(t)
	sender, err := iotxaddress.NewAddress(true, chainid)
	require.Nil(err)
	recipient, err := iotxaddress.NewAddress(true, chainid)
	require.Nil(err)
	v := NewVote(0, sender.PublicKey, recipient.PublicKey)

	signedv, err := v.Sign(sender)
	require.Nil(err)
	require.Nil(signedv.Verify(sender))
	require.NotNil(signedv.Verify(recipient))
}

func TestVoteSerializedDeserialize(t *testing.T) {
	require := require.New(t)
	sender, err := iotxaddress.NewAddress(true, chainid)
	require.Nil(err)
	recipient, err := iotxaddress.NewAddress(true, chainid)
	require.Nil(err)

	v := NewVote(0, sender.PublicKey, recipient.PublicKey)
	raw, err := v.Serialize()
	require.Nil(err)

	newv := &Vote{}
	require.Nil(newv.Deserialize(raw))
	require.Equal(v.Hash(), newv.Hash())
	require.Equal(v.TotalSize(), newv.TotalSize())
}
