// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"context"
	"strconv"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blockdao"
)

func TestSyncIndexers_StartHeight(t *testing.T) {
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
					mockIndexerWithStart.EXPECT().Start(gomock.Any()).Return(nil).Times(1)
					mockIndexerWithStart.EXPECT().StartHeight().Return(indexer[0]).Times(1)
					mockIndexerWithStart.EXPECT().Height().Return(indexer[1], nil).Times(1)
					indexers = append(indexers, mockIndexerWithStart)
				} else {
					mockIndexer := mock_blockdao.NewMockBlockIndexer(ctrl)
					mockIndexer.EXPECT().Start(gomock.Any()).Return(nil).Times(1)
					mockIndexer.EXPECT().Height().Return(indexer[1], nil).Times(1)
					indexers = append(indexers, mockIndexer)
				}
			}
			ig := NewSyncIndexers(indexers...)
			err := ig.Start(context.Background())
			require.NoError(err)
			height := ig.StartHeight()
			require.Equal(c.expect, height)
		})
	}

}

func TestSyncIndexers_Height(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cases := []struct {
		heights []uint64
		expect  uint64
	}{
		{[]uint64{}, 0},
		{[]uint64{100}, 100},
		{[]uint64{100, 100}, 100},
		{[]uint64{90, 100}, 90},
		{[]uint64{100, 90}, 90},
		{[]uint64{100, 100, 100}, 100},
		{[]uint64{90, 100, 100}, 90},
		{[]uint64{90, 80, 100}, 80},
		{[]uint64{90, 80, 70}, 70},
	}

	for i := range cases {
		name := strconv.FormatUint(uint64(i), 10)
		t.Run(name, func(t *testing.T) {
			var indexers []blockdao.BlockIndexer
			for _, height := range cases[i].heights {
				mockIndexer := mock_blockdao.NewMockBlockIndexer(ctrl)
				mockIndexer.EXPECT().Height().Return(height, nil).Times(1)
				indexers = append(indexers, mockIndexer)
			}
			ig := NewSyncIndexers(indexers...)
			height, err := ig.Height()
			require.NoError(err)
			require.Equal(cases[i].expect, height)
		})
	}
}

func TestSyncIndexers_PutBlock(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cases := []struct {
		indexers     [][2]uint64 // [startHeight, height]
		blocks       []uint64    // blocks to put
		expectBlocks [][]uint64  // expect blocks to put on every indexer
	}{
		{
			[][2]uint64{},
			[]uint64{100},
			[][]uint64{},
		},
		{
			[][2]uint64{{100, 10}},
			[]uint64{10, 20, 90, 100, 101},
			[][]uint64{{100, 101}},
		},
		{
			[][2]uint64{{100, 210}},
			[]uint64{10, 20, 90, 100, 101, 210, 211},
			[][]uint64{{211}},
		},
		{
			[][2]uint64{{0, 200}, {250, 1}},
			[]uint64{1, 2, 201, 249, 250, 251},
			[][]uint64{{201, 249, 250, 251}, {250, 251}},
		},
		{
			[][2]uint64{{0, 250}, {250, 250}},
			[]uint64{1, 2, 201, 249, 250, 251, 252},
			[][]uint64{{251, 252}, {251, 252}},
		},
		{
			[][2]uint64{{0, 200}, {250, 1}, {300, 1}},
			[]uint64{1, 2, 201, 249, 250, 251, 300, 301},
			[][]uint64{{201, 249, 250, 251, 300, 301}, {250, 251, 300, 301}, {300, 301}},
		},
		{
			[][2]uint64{{0, 250}, {250, 250}, {300, 250}},
			[]uint64{1, 2, 201, 249, 250, 251, 300, 301},
			[][]uint64{{251, 300, 301}, {251, 300, 301}, {300, 301}},
		},
		{
			[][2]uint64{{0, 300}, {250, 300}, {300, 300}},
			[]uint64{1, 2, 201, 249, 250, 251, 300, 301},
			[][]uint64{{301}, {301}, {301}},
		},
		{
			[][2]uint64{{0, 400}, {250, 400}, {300, 400}},
			[]uint64{1, 2, 201, 249, 250, 251, 300, 301, 400, 401},
			[][]uint64{{401}, {401}, {401}},
		},
	}

	for _, c := range cases {
		t.Run("", func(t *testing.T) {
			var indexers []blockdao.BlockIndexer
			putBlocks := make([][]uint64, len(c.indexers))
			indexersHeight := make([]uint64, len(c.indexers))
			for id, indexer := range c.indexers {
				idx := id
				indexersHeight[idx] = indexer[1]
				if indexer[0] > 0 {
					mockIndexerWithStart := mock_blockdao.NewMockBlockIndexerWithStart(ctrl)
					mockIndexerWithStart.EXPECT().Start(gomock.Any()).Return(nil).Times(1)
					mockIndexerWithStart.EXPECT().StartHeight().Return(indexer[0]).Times(1)
					mockIndexerWithStart.EXPECT().Height().DoAndReturn(func() (uint64, error) {
						return indexersHeight[idx], nil
					}).AnyTimes()
					mockIndexerWithStart.EXPECT().PutBlock(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, blk *block.Block) error {
						putBlocks[idx] = append(putBlocks[idx], blk.Height())
						indexersHeight[idx] = blk.Height()
						return nil
					}).Times(len(c.expectBlocks[idx]))
					indexers = append(indexers, mockIndexerWithStart)
				} else {
					mockIndexer := mock_blockdao.NewMockBlockIndexer(ctrl)
					mockIndexer.EXPECT().Start(gomock.Any()).Return(nil).Times(1)
					mockIndexer.EXPECT().Height().DoAndReturn(func() (uint64, error) {
						return indexersHeight[idx], nil
					}).AnyTimes()
					mockIndexer.EXPECT().PutBlock(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, blk *block.Block) error {
						putBlocks[idx] = append(putBlocks[idx], blk.Height())
						indexersHeight[idx] = blk.Height()
						return nil
					}).Times(len(c.expectBlocks[idx]))
					indexers = append(indexers, mockIndexer)
				}
			}
			ig := NewSyncIndexers(indexers...)
			err := ig.Start(context.Background())
			require.NoError(err)
			for _, blkHeight := range c.blocks {
				blk, err := block.NewBuilder(block.RunnableActions{}).SetHeight(blkHeight).SignAndBuild(identityset.PrivateKey(0))
				require.NoError(err)
				err = ig.PutBlock(context.Background(), &blk)
				require.NoError(err)
			}
			require.Equal(c.expectBlocks, putBlocks)
		})
	}
}
