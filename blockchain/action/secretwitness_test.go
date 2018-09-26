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

func TestSecretWitnessSerializedDeserialize(t *testing.T) {
	require := require.New(t)
	sender, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)

	sw, err := NewSecretWitness(0, sender.RawAddress, [][]byte{{1, 2, 3}, {4, 5, 6}})
	require.NoError(err)
	raw, err := sw.Serialize()
	require.NoError(err)

	newSw := &SecretWitness{}
	require.NoError(newSw.Deserialize(raw))
	require.Equal(sw.Hash(), newSw.Hash())
}
