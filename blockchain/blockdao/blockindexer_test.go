// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"
	"strconv"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockdao"
)

func TestCheckIndexer(t *testing.T) {

	cases := []struct {
		daoHeight         uint64
		indexerTipHeight  uint64
		expectedPutBlocks []uint64
		noErr             bool
	}{
		{5, 0, []uint64{1, 2, 3, 4, 5}, true},
		{5, 1, []uint64{2, 3, 4, 5}, true},
		{5, 2, []uint64{3, 4, 5}, true},
		{5, 3, []uint64{4, 5}, true},
		{5, 4, []uint64{5}, true},
		{5, 5, []uint64{}, true},
		{5, 6, []uint64{}, false},
	}

	for i, c := range cases {
		t.Run(strconv.FormatUint(uint64(i), 10), func(t *testing.T) {
			require := require.New(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockDao := mock_blockdao.NewMockBlockDAO(ctrl)
			checker := NewBlockIndexerChecker(mockDao)
			indexer := mock_blockdao.NewMockBlockIndexer(ctrl)

			putBlocks := make([]*block.Block, 0)
			mockDao.EXPECT().Height().Return(c.daoHeight, nil).Times(1)
			mockDao.EXPECT().GetBlockByHeight(gomock.Any()).DoAndReturn(func(arg0 uint64) (*block.Block, error) {
				pb := &iotextypes.BlockHeader{
					Core: &iotextypes.BlockHeaderCore{
						Height:    arg0,
						Timestamp: timestamppb.Now(),
					},
					ProducerPubkey: identityset.PrivateKey(1).PublicKey().Bytes(),
				}
				blk := &block.Block{}
				err := blk.LoadFromBlockHeaderProto(pb)
				return blk, err
			}).AnyTimes()
			mockDao.EXPECT().GetReceipts(gomock.Any()).Return(nil, nil).AnyTimes()
			indexer.EXPECT().Height().Return(c.indexerTipHeight, nil).Times(1)
			indexer.EXPECT().PutBlock(gomock.Any(), gomock.Any()).DoAndReturn(func(arg0 context.Context, arg1 *block.Block) error {
				putBlocks = append(putBlocks, arg1)
				return nil
			}).AnyTimes()

			ctx := protocol.WithBlockchainCtx(context.Background(), protocol.BlockchainCtx{})
			ctx = genesis.WithGenesisContext(ctx, genesis.Default)
			err := checker.CheckIndexer(ctx, indexer, 0, func(u uint64) {})
			require.Equalf(c.noErr, err == nil, "error: %v", err)
			require.Len(putBlocks, len(c.expectedPutBlocks))
			for k, h := range c.expectedPutBlocks {
				require.Equal(h, putBlocks[k].Height())
			}
		})
	}
}

func TestCheckIndexerWithStart(t *testing.T) {

	cases := []struct {
		daoHeight          uint64
		indexerTipHeight   uint64
		indexerStartHeight uint64
		expectedPutBlocks  []uint64
		noErr              bool
	}{
		{5, 0, 3, []uint64{3, 4, 5}, true},
		{5, 1, 3, []uint64{3, 4, 5}, true},
		{5, 2, 3, []uint64{3, 4, 5}, true},
		{5, 3, 3, []uint64{4, 5}, true},
		{5, 4, 3, []uint64{5}, true},
		{5, 5, 3, []uint64{}, true},
		{5, 6, 3, []uint64{}, false},
	}

	for i, c := range cases {
		t.Run(strconv.FormatUint(uint64(i), 10), func(t *testing.T) {
			require := require.New(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockDao := mock_blockdao.NewMockBlockDAO(ctrl)
			checker := NewBlockIndexerChecker(mockDao)
			indexer := mock_blockdao.NewMockBlockIndexerWithStart(ctrl)

			putBlocks := make([]*block.Block, 0)
			mockDao.EXPECT().Height().Return(c.daoHeight, nil).Times(1)
			mockDao.EXPECT().GetBlockByHeight(gomock.Any()).DoAndReturn(func(arg0 uint64) (*block.Block, error) {
				pb := &iotextypes.BlockHeader{
					Core: &iotextypes.BlockHeaderCore{
						Height:    arg0,
						Timestamp: timestamppb.Now(),
					},
					ProducerPubkey: identityset.PrivateKey(1).PublicKey().Bytes(),
				}
				blk := &block.Block{}
				err := blk.LoadFromBlockHeaderProto(pb)
				return blk, err
			}).AnyTimes()
			mockDao.EXPECT().GetReceipts(gomock.Any()).Return(nil, nil).AnyTimes()
			indexer.EXPECT().Height().Return(c.indexerTipHeight, nil).Times(1)
			indexer.EXPECT().StartHeight().Return(c.indexerStartHeight).AnyTimes()
			indexer.EXPECT().PutBlock(gomock.Any(), gomock.Any()).DoAndReturn(func(arg0 context.Context, arg1 *block.Block) error {
				putBlocks = append(putBlocks, arg1)
				return nil
			}).AnyTimes()

			ctx := protocol.WithBlockchainCtx(context.Background(), protocol.BlockchainCtx{})
			ctx = genesis.WithGenesisContext(ctx, genesis.Default)
			err := checker.CheckIndexer(ctx, indexer, 0, func(u uint64) {})
			require.Equalf(c.noErr, err == nil, "error: %v", err)
			require.Len(putBlocks, len(c.expectedPutBlocks))
			for k, h := range c.expectedPutBlocks {
				require.Equal(h, putBlocks[k].Height())
			}
		})
	}
}
