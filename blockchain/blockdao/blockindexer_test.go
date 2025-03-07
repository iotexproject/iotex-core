// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blockdao"
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
			checker := NewBlockIndexerChecker(mockDao, nil)
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
			mockDao.EXPECT().HeaderByHeight(gomock.Any()).DoAndReturn(func(arg0 uint64) (*block.Header, error) {
				pb := &iotextypes.BlockHeader{
					Core: &iotextypes.BlockHeaderCore{
						Height:    arg0,
						Timestamp: timestamppb.Now(),
					},
					ProducerPubkey: identityset.PrivateKey(1).PublicKey().Bytes(),
				}
				blk := &block.Block{}
				err := blk.LoadFromBlockHeaderProto(pb)
				return &blk.Header, err
			}).AnyTimes()
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
			checker := NewBlockIndexerChecker(mockDao, nil)
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
			mockDao.EXPECT().HeaderByHeight(gomock.Any()).DoAndReturn(func(arg0 uint64) (*block.Header, error) {
				pb := &iotextypes.BlockHeader{
					Core: &iotextypes.BlockHeaderCore{
						Height:    arg0,
						Timestamp: timestamppb.Now(),
					},
					ProducerPubkey: identityset.PrivateKey(1).PublicKey().Bytes(),
				}
				blk := &block.Block{}
				err := blk.LoadFromBlockHeaderProto(pb)
				return &blk.Header, err
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

func TestBlockIndexerChecker_CheckIndexer(t *testing.T) {
	r := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	store := mock_blockdao.NewMockBlockDAO(ctrl)
	dao := &blockDAO{blockStore: store}
	bic := NewBlockIndexerChecker(dao, nil)
	indexer := mock_blockdao.NewMockBlockIndexer(ctrl)

	t.Run("WithoutBlockchainContext", func(t *testing.T) {
		err := bic.CheckIndexer(ctx, indexer, 0, nil)
		r.ErrorContains(err, "failed to find blockchain ctx")
	})

	t.Run("WithoutGenesisContext", func(t *testing.T) {
		ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{})

		err := bic.CheckIndexer(ctx, indexer, 0, nil)
		r.ErrorContains(err, "failed to find genesis ctx")
	})

	t.Run("FailedToGetIndexerHeight", func(t *testing.T) {
		ctx = genesis.WithGenesisContext(ctx, genesis.Genesis{})

		indexer.EXPECT().Height().Return(uint64(0), errors.New(t.Name())).Times(1)

		err := bic.CheckIndexer(ctx, indexer, 0, nil)
		r.ErrorContains(err, t.Name())
	})

	t.Run("FailedToGetDaoTipHeight", func(t *testing.T) {
		indexer.EXPECT().Height().Return(uint64(1), nil).Times(1)
		store.EXPECT().Height().Return(uint64(0), errors.New(t.Name())).Times(1)

		err := bic.CheckIndexer(ctx, indexer, 0, nil)
		r.ErrorContains(err, t.Name())
	})

	t.Run("IndexerTipHeightHigherThanDaoTipHeight", func(t *testing.T) {
		tipHeight := uint64(100)
		daoTip := uint64(99)

		indexer.EXPECT().Height().Return(tipHeight, nil).Times(1)
		store.EXPECT().Height().Return(daoTip, nil).Times(1)

		err := bic.CheckIndexer(ctx, indexer, 0, nil)
		r.ErrorContains(err, "indexer tip height cannot by higher than dao tip height")
	})

	t.Run("FailedToGetBlockByHeight", func(t *testing.T) {
		tipHeight := uint64(98)
		daoTip := uint64(99)

		indexer.EXPECT().Height().Return(tipHeight, nil).Times(1)
		store.EXPECT().Height().Return(daoTip, nil).Times(1)
		store.EXPECT().GetBlockByHeight(gomock.Any()).Return(nil, errors.New(t.Name())).Times(1)

		err := bic.CheckIndexer(ctx, indexer, 0, nil)
		r.ErrorContains(err, t.Name())
	})

	t.Run("LoopFromStartHeightToTargetHeight", func(t *testing.T) {
		tipHeight := uint64(98)
		daoTip := uint64(99)

		t.Run("FailedToGetBlockByHeight", func(t *testing.T) {
			indexer.EXPECT().Height().Return(tipHeight, nil).Times(1)
			store.EXPECT().Height().Return(daoTip, nil).Times(1)
			store.EXPECT().GetBlockByHeight(gomock.Any()).Return(&block.Block{}, nil).Times(1)
			store.EXPECT().GetBlockByHeight(gomock.Any()).Return(nil, errors.New(t.Name())).Times(1)

			err := bic.CheckIndexer(ctx, indexer, 0, nil)
			r.ErrorContains(err, t.Name())
		})

		t.Run("FailedToGetReceipts", func(t *testing.T) {
			indexer.EXPECT().Height().Return(tipHeight, nil).Times(1)
			store.EXPECT().Height().Return(daoTip, nil).Times(1)
			store.EXPECT().GetBlockByHeight(gomock.Any()).Return(&block.Block{}, nil).Times(2)
			store.EXPECT().GetReceipts(gomock.Any()).Return(nil, errors.New(t.Name())).Times(1)

			err := bic.CheckIndexer(ctx, indexer, 0, nil)
			r.ErrorContains(err, t.Name())
		})

		t.Run("FailedToGetPubKey", func(t *testing.T) {
			indexer.EXPECT().Height().Return(tipHeight, nil).Times(1)
			store.EXPECT().Height().Return(daoTip, nil).Times(1)
			store.EXPECT().GetBlockByHeight(gomock.Any()).Return(&block.Block{
				Header: block.Header{},
			}, nil).Times(2)
			store.EXPECT().GetReceipts(gomock.Any()).Return([]*action.Receipt{}, nil).Times(1)

			err := bic.CheckIndexer(ctx, indexer, 0, nil)
			r.ErrorContains(err, "failed to get pubkey")
		})

		pubkey, _ := crypto.HexStringToPublicKey("04806b217cb0b6a675974689fd99549e525d967287eee9a62dc4e598eea981b8158acfe026da7bf58397108abd0607672832c28ef3bc7b5855077f6e67ab5fc096")

		t.Run("FailedToGetAddress", func(t *testing.T) {
			indexer.EXPECT().Height().Return(tipHeight, nil).Times(1)
			store.EXPECT().Height().Return(daoTip, nil).Times(1)
			store.EXPECT().GetBlockByHeight(gomock.Any()).Return(&block.Block{}, nil).Times(2)
			store.EXPECT().GetReceipts(gomock.Any()).Return([]*action.Receipt{}, nil).Times(1)

			p := gomonkey.NewPatches()
			defer p.Reset()

			p.ApplyMethodReturn(&block.Header{}, "PublicKey", pubkey)
			p.ApplyMethodReturn(pubkey, "Address", nil)

			err := bic.CheckIndexer(ctx, indexer, 0, nil)
			r.ErrorContains(err, "failed to get producer address")
		})
	})
}
