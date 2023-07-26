// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockdao"
)

func TestIndexerGroup_StartHeight(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	cases := []struct {
		name     string
		indexers [][2]uint64 // [startHeight, height]
		expect   uint64
	}{
		{"no indexers", nil, 0},
		{"one indexer without start height", [][2]uint64{{0, 100}}, 101},
		{"one indexer with start height I", [][2]uint64{{100, 200}}, 201},
		{"one indexer with start height II", [][2]uint64{{300, 200}}, 300},
		{"two indexers with start height I", [][2]uint64{{100, 200}, {200, 300}}, 201},
		{"two indexers with start height II", [][2]uint64{{100, 200}, {400, 300}}, 201},
		{"two indexers with start height III", [][2]uint64{{100, 350}, {400, 300}}, 351},
		{"two indexers one with start height I", [][2]uint64{{0, 1}, {150, 1}}, 2},
		{"two indexers one with start height II", [][2]uint64{{0, 1}, {150, 200}}, 2},
		{"two indexers one with start height III", [][2]uint64{{0, 200}, {250, 1}}, 201},
		{"two indexers one with start height IV", [][2]uint64{{0, 200}, {150, 1}}, 150},
		{"two indexers I", [][2]uint64{{0, 5}, {0, 1}}, 2},
		{"two indexers II", [][2]uint64{{0, 5}, {0, 5}}, 6},
		{"two indexers III", [][2]uint64{{0, 5}, {0, 6}}, 6},
		{"multiple indexers I", [][2]uint64{{0, 5}, {0, 6}, {0, 7}}, 6},
		{"multiple indexers II", [][2]uint64{{0, 5}, {10, 6}, {0, 7}}, 6},
		{"multiple indexers III", [][2]uint64{{10, 5}, {0, 6}, {0, 7}}, 7},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var indexers []blockdao.BlockIndexer
			for _, indexer := range c.indexers {
				if indexer[0] > 0 {
					mockIndexerWithStart := mock_blockdao.NewMockBlockIndexerWithStart(ctrl)
					mockIndexerWithStart.EXPECT().StartHeight().Return(indexer[0]).Times(1)
					mockIndexerWithStart.EXPECT().Height().Return(indexer[1], nil).Times(1)
					indexers = append(indexers, mockIndexerWithStart)
				} else {
					mockIndexer := mock_blockdao.NewMockBlockIndexer(ctrl)
					mockIndexer.EXPECT().Height().Return(indexer[1], nil).Times(1)
					indexers = append(indexers, mockIndexer)
				}
			}
			ig := NewIndexerGroup(indexers...)
			height, err := ig.StartHeight()
			require.NoError(err)
			require.Equal(c.expect, height)
		})
	}

}
