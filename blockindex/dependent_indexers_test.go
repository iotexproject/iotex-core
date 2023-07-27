// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"strconv"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockdao"
)

func TestDependentIndexers(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	index1 := mock_blockdao.NewMockBlockIndexer(ctrl)
	index2 := mock_blockdao.NewMockBlockIndexer(ctrl)
	index3 := mock_blockdao.NewMockBlockIndexer(ctrl)

	cases := []struct {
		dependencies [][2]blockdao.BlockIndexer
		expect       []blockdao.BlockIndexer
	}{
		{[][2]blockdao.BlockIndexer{{index1, index2}, {index1, index3}}, []blockdao.BlockIndexer{index1, index2, index3}},
		{[][2]blockdao.BlockIndexer{{index1, index2}, {index2, index3}}, []blockdao.BlockIndexer{index1, index2, index3}},
		{[][2]blockdao.BlockIndexer{{index1, index2}, {index3, index2}}, []blockdao.BlockIndexer{index1, index3, index2}},
	}

	for i, c := range cases {
		name := strconv.FormatInt(int64(i), 10)
		t.Run(name, func(t *testing.T) {
			dependentIndexers, err := NewDependentIndexers(c.dependencies...)
			require.NoError(err)
			require.Equal(len(c.expect), len(dependentIndexers.indexers))
			for i := range c.expect {
				require.True(c.expect[i] == dependentIndexers.indexers[i])
			}
		})
	}
}
