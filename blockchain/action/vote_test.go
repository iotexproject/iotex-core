// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
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
	require.NoError(err)
	recipient, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)
	v, err := NewVote(0, sender.RawAddress, recipient.RawAddress, uint64(100000), uint64(10))
	require.NoError(err)

	signedv, err := v.Sign(sender)
	require.NoError(err)
	require.NoError(signedv.Verify(sender))
	require.NotNil(signedv.Verify(recipient))
}

func TestVoteSerializedDeserialize(t *testing.T) {
	require := require.New(t)
	sender, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)
	recipient, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)

	v, err := NewVote(0, sender.RawAddress, recipient.RawAddress, uint64(100000), uint64(10))
	require.NoError(err)
	raw, err := v.Serialize()
	require.NoError(err)

	newv := &Vote{}
	require.NoError(newv.Deserialize(raw))
	require.Equal(v.Hash(), newv.Hash())
	require.Equal(v.TotalSize(), newv.TotalSize())
}

func TestVoteToJSONFromJSON(t *testing.T) {
	require := require.New(t)
	sender, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)
	recipient, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)

	v, err := NewVote(0, sender.RawAddress, recipient.RawAddress, uint64(100000), uint64(10))
	require.NoError(err)
	signedv, err := v.Sign(sender)
	require.NoError(err)

	expv, err := signedv.ToJSON()
	require.NoError(err)
	require.NotNil(expv)

	newv, err := NewVoteFromJSON(expv)
	require.NoError(err)
	require.NotNil(newv)

	require.NoError(newv.Verify(sender))
	require.NotNil(newv.Verify(recipient))
	require.Equal(v.Hash(), newv.Hash())
	require.Equal(v.TotalSize(), newv.TotalSize())
}
