// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestEpochMetaSerializeAndDeserialize(t *testing.T) {
	require := require.New(t)

	bm1 := NewBlockMeta(uint64(1), identityset.Address(1).String(), testutil.TimestampNow())
	ss, err := bm1.Serialize()
	require.NoError(err)
	bm2 := &BlockMeta{}
	require.NoError(bm2.Deserialize(ss))
	require.Equal(bm1.Height, bm2.Height)
	require.Equal(bm1.Producer, bm2.Producer)
	require.Equal(bm1.MintTime, bm2.MintTime)
}
