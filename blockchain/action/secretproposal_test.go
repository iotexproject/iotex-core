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

func TestSecretProposalSerializedDeserialize(t *testing.T) {
	require := require.New(t)
	sender, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)
	recipient, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)

	sp, err := NewSecretProposal(0, sender.RawAddress, recipient.RawAddress, []uint32{1, 2, 3})
	require.NoError(err)
	raw, err := sp.Serialize()
	require.NoError(err)

	newSp := &SecretProposal{}
	require.NoError(newSp.Deserialize(raw))
	require.Equal(sp.Hash(), newSp.Hash())
}
