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
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blockdao"
)

// checkIndexerFixture is the mock wiring the two table-driven CheckIndexer tests
// share. They differ only in what they assert -- TestCheckIndexer covers the
// indexer-ahead-of-DAO error, TestCheckIndexerTargetHeight the targetHeight cap
// -- so the setup lives here rather than being written out twice.
type checkIndexerFixture struct {
	checker   BlockIndexerChecker
	indexer   *mock_blockdao.MockBlockIndexer
	ctx       context.Context
	putBlocks *[]*block.Block
}

// newCheckIndexerFixture serves a synthetic header for any height the checker
// asks for and records every block handed to the indexer.
//
// putBlocks is appended to from inside the PutBlock expectation, so read it only
// after CheckIndexer has returned.
func newCheckIndexerFixture(t *testing.T, daoHeight, indexerTipHeight uint64) *checkIndexerFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockDao := mock_blockdao.NewMockBlockDAO(ctrl)
	indexer := mock_blockdao.NewMockBlockIndexer(ctrl)
	putBlocks := make([]*block.Block, 0)

	blockAt := func(height uint64) (*block.Block, error) {
		pb := &iotextypes.BlockHeader{
			Core: &iotextypes.BlockHeaderCore{
				Height:    height,
				Timestamp: timestamppb.Now(),
			},
			ProducerPubkey: identityset.PrivateKey(1).PublicKey().Bytes(),
		}
		blk := &block.Block{}
		err := blk.LoadFromBlockHeaderProto(pb)
		return blk, err
	}

	mockDao.EXPECT().Height().Return(daoHeight, nil).Times(1)
	mockDao.EXPECT().GetBlockByHeight(gomock.Any()).DoAndReturn(blockAt).AnyTimes()
	mockDao.EXPECT().GetReceipts(gomock.Any()).Return(nil, nil).AnyTimes()
	mockDao.EXPECT().HeaderByHeight(gomock.Any()).DoAndReturn(func(height uint64) (*block.Header, error) {
		blk, err := blockAt(height)
		return &blk.Header, err
	}).AnyTimes()
	indexer.EXPECT().Height().Return(indexerTipHeight, nil).Times(1)
	indexer.EXPECT().PutBlock(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, blk *block.Block) error {
		putBlocks = append(putBlocks, blk)
		return nil
	}).AnyTimes()

	ctx := protocol.WithBlockchainCtx(context.Background(), protocol.BlockchainCtx{})
	ctx = genesis.WithGenesisContext(ctx, genesis.TestDefault())

	return &checkIndexerFixture{
		checker:   NewBlockIndexerChecker(mockDao),
		indexer:   indexer,
		ctx:       ctx,
		putBlocks: &putBlocks,
	}
}

func (f *checkIndexerFixture) requirePutHeights(t *testing.T, expected []uint64) {
	t.Helper()
	require := require.New(t)
	require.Len(*f.putBlocks, len(expected))
	for i, h := range expected {
		require.Equal(h, (*f.putBlocks)[i].Height())
	}
}

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
			f := newCheckIndexerFixture(t, c.daoHeight, c.indexerTipHeight)
			err := f.checker.CheckIndexer(f.ctx, f.indexer, 0, func(uint64) {})
			require.Equalf(t, c.noErr, err == nil, "error: %v", err)
			f.requirePutHeights(t, c.expectedPutBlocks)
		})
	}
}

// TestCheckIndexerTargetHeight covers the targetHeight cap, which TestCheckIndexer
// above never exercises (it always passes 0). The cap is what makes a segmented
// replay stop exactly on its boundary instead of running to the block DAO tip.
func TestCheckIndexerTargetHeight(t *testing.T) {
	cases := []struct {
		name              string
		daoHeight         uint64
		indexerTipHeight  uint64
		targetHeight      uint64
		expectedPutBlocks []uint64
	}{
		{"zero means no cap", 5, 0, 0, []uint64{1, 2, 3, 4, 5}},
		{"cap below dao tip truncates", 5, 0, 3, []uint64{1, 2, 3}},
		{"cap below dao tip from a warm indexer", 5, 2, 3, []uint64{3}},
		{"cap equal to dao tip", 5, 0, 5, []uint64{1, 2, 3, 4, 5}},
		{"cap above dao tip falls back to dao tip", 5, 0, 9, []uint64{1, 2, 3, 4, 5}},
		{"cap already reached is a no-op", 5, 3, 3, []uint64{}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newCheckIndexerFixture(t, c.daoHeight, c.indexerTipHeight)
			require.NoError(t, f.checker.CheckIndexer(f.ctx, f.indexer, c.targetHeight, func(uint64) {}))
			f.requirePutHeights(t, c.expectedPutBlocks)
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
	bic := NewBlockIndexerChecker(dao)
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
			store.EXPECT().TransactionLogs(gomock.Any()).Return(&iotextypes.TransactionLogs{Logs: nil}, nil).AnyTimes()

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
