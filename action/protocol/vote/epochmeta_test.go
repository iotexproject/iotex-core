// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package vote

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestEpochMetaSerializeAndDeserialize(t *testing.T) {
	require := require.New(t)

	epochNum := uint64(100)
	em := NewEpochMeta(epochNum)
	em.AddBlockMeta(uint64(1), identityset.Address(1).String(), testutil.TimestampNow())
	em.AddBlockMeta(uint64(2), identityset.Address(2).String(), testutil.TimestampNow())
	em.AddBlockMeta(uint64(3), identityset.Address(3).String(), testutil.TimestampNow())

	ss, err := em.Serialize() 
	require.NoError(err)
	em2 := &EpochMeta{}
	require.NoError(em2.Deserialize(ss))
	require.Equal(em.EpochNumber, em2.EpochNumber)
	for i, blockmeta := range em.BlockMetas {
		require.Equal(blockmeta.Height, em2.BlockMetas[i].Height)
		require.Equal(blockmeta.Producer, em2.BlockMetas[i].Producer)
		require.Equal(blockmeta.MintTime, em2.BlockMetas[i].MintTime)
	}
}