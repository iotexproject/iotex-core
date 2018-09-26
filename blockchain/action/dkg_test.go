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

func TestDKGSerializedDeserialize(t *testing.T) {
	require := require.New(t)
	sender, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)
	recipient, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)

	dkg, err := NewDKG(0, sender.RawAddress, recipient.RawAddress, []uint32{1, 2, 3}, []byte{4,5,6})
	require.NoError(err)
	raw, err := dkg.Serialize()
	require.NoError(err)

	newDKG := &DKG{}
	require.NoError(newDKG.Deserialize(raw))
	require.Equal(dkg.Hash(), newDKG.Hash())
}
