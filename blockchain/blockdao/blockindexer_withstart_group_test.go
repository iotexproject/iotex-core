// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"strconv"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain/blockdao/mock"
)

func TestBlockIndexerWithStartGroup_Height(t *testing.T) {
	require := require.New(t)

	t.Run("none indexer", func(t *testing.T) {
		// Create a new BlockIndexerWithStartGroup with two mock indexers
		tipHeight := uint64(0)
		maxValidHeightFunc := func() (uint64, error) { return tipHeight, nil }
		group := NewBlockIndexerWithStartGroup()
		group.SetTipHeightFunc(maxValidHeightFunc)

		tests := []struct {
			tipHeight uint64
			expect    uint64
		}{
			{0, 0},
			{1, 1},
			{2, 2},
			{8, 8},
			{9, 9},
			{10, 10},
			{11, 11},
			{12, 12},
		}

		for _, test := range tests {
			tipHeight = test.tipHeight
			height, err := group.Height()
			require.NoError(err)
			require.Equal(test.expect, height)
		}
	})

	t.Run("one indexer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		// Create a new BlockIndexerWithStartGroup with two mock indexers
		indexer1 := mock.NewMockBlockIndexerWithStart(ctrl)
		indexers := []BlockIndexerWithStart{indexer1}
		tipHeight := uint64(0)
		maxValidHeightFunc := func() (uint64, error) { return tipHeight, nil }
		group := NewBlockIndexerWithStartGroup(indexers...)
		group.SetTipHeightFunc(maxValidHeightFunc)

		tests := []struct {
			startHeight uint64
			height      uint64
			tipHeight   uint64
			expect      uint64
			noErr       bool
		}{
			// indexer is empty
			{10, 0, 0, 0, true},
			{10, 0, 1, 1, true},
			{10, 0, 2, 2, true},
			{10, 0, 8, 8, true},
			{10, 0, 9, 9, true},
			{10, 0, 10, 9, true},
			{10, 0, 11, 9, true},
			{10, 0, 12, 9, true},
			// indexer is not empty
			{10, 10, 10, 10, true},
			{10, 11, 11, 11, true},
			// indexer is higher than tip height
			{10, 11, 10, 0, false},
		}

		for i, test := range tests {
			t.Run(strconv.FormatInt(int64(i), 10), func(t *testing.T) {
				tipHeight = test.tipHeight
				if test.noErr {
					indexer1.EXPECT().StartHeight().Return(test.startHeight).Times(1)
				}
				indexer1.EXPECT().Height().Return(test.height, nil).Times(1)
				height, err := group.Height()
				require.Equal(test.noErr, err == nil)
				require.Equal(test.expect, height)
			})
		}
	})

	t.Run("multiple indexers", func(t *testing.T) {
		type indexerConfig struct {
			startHeight uint64
			height      uint64
		}
		tests := []struct {
			cfgs      []indexerConfig
			tipHeight uint64
			expect    uint64
		}{
			// indexer is empty
			{[]indexerConfig{{10, 0}, {20, 0}}, 0, 0},
			{[]indexerConfig{{10, 0}, {20, 0}}, 1, 1},
			{[]indexerConfig{{10, 0}, {20, 0}}, 2, 2},
			{[]indexerConfig{{10, 0}, {20, 0}}, 8, 8},
			{[]indexerConfig{{10, 0}, {20, 0}}, 9, 9},
			{[]indexerConfig{{10, 0}, {20, 0}}, 10, 9},
			// one indexer is not empty
			{[]indexerConfig{{10, 10}, {20, 0}}, 11, 10},
			{[]indexerConfig{{10, 11}, {20, 0}}, 12, 11},
			{[]indexerConfig{{10, 18}, {20, 0}}, 19, 18},
			{[]indexerConfig{{10, 19}, {20, 0}}, 20, 19},
			// two indexers are not empty
			{[]indexerConfig{{10, 20}, {20, 20}}, 21, 20},
			{[]indexerConfig{{10, 21}, {20, 21}}, 22, 21},
			// add a new indexer
			{[]indexerConfig{{10, 21}, {20, 21}, {5, 0}}, 23, 4},
			{[]indexerConfig{{10, 21}, {20, 21}, {15, 0}}, 23, 14},
			{[]indexerConfig{{10, 21}, {20, 21}, {20, 0}}, 23, 19},
			{[]indexerConfig{{10, 21}, {20, 21}, {30, 0}}, 23, 21},
		}

		for i, test := range tests {
			t.Run(strconv.FormatInt(int64(i), 10), func(t *testing.T) {
				ctrl := gomock.NewController(t)
				indexers := make([]*mock.MockBlockIndexerWithStart, len(test.cfgs))
				indexers2 := make([]BlockIndexerWithStart, len(test.cfgs))
				for j := range test.cfgs {
					indexers[j] = mock.NewMockBlockIndexerWithStart(ctrl)
					indexers2[j] = indexers[j]
				}
				tipHeight := uint64(0)
				maxValidHeightFunc := func() (uint64, error) { return tipHeight, nil }
				group := NewBlockIndexerWithStartGroup(indexers2...)
				group.SetTipHeightFunc(maxValidHeightFunc)

				tipHeight = test.tipHeight
				for j, indexer := range indexers {
					indexer.EXPECT().StartHeight().Return(test.cfgs[j].startHeight).Times(1)
					indexer.EXPECT().Height().Return(test.cfgs[j].height, nil).Times(1)
				}
				height, err := group.Height()
				require.NoError(err)
				require.Equal(test.expect, height)
			})
		}
	})
}
