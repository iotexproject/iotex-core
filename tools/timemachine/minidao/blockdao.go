// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package minidao

import (
	"context"
	"sync/atomic"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/filedao"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
)

type (
	blockDAO struct {
		blockStore filedao.FileDAO
		indexers   []blockdao.BlockIndexer
		lifecycle  lifecycle.Lifecycle
		tipHeight  uint64
	}
)

// NewBlockDAO instantiates a block DAO
func NewBlockDAO(indexers []blockdao.BlockIndexer, cfg db.Config, deser *block.Deserializer) blockdao.BlockDAO {
	blkStore, err := filedao.NewFileDAO(cfg, deser)
	if err != nil {
		log.L().Fatal(err.Error(), zap.Any("cfg", cfg))
		return nil
	}
	return createBlockDAO(blkStore, indexers, cfg)
}

func createBlockDAO(blkStore filedao.FileDAO, indexers []blockdao.BlockIndexer, cfg db.Config) blockdao.BlockDAO {
	if blkStore == nil {
		return nil
	}

	blockDAO := &blockDAO{
		blockStore: blkStore,
		indexers:   indexers,
	}

	blockDAO.lifecycle.Add(blkStore)
	for _, indexer := range indexers {
		blockDAO.lifecycle.Add(indexer)
	}
	return blockDAO
}

// Start starts block DAO and initiates the top height if it doesn't exist
func (dao *blockDAO) Start(ctx context.Context) error {
	err := dao.lifecycle.OnStart(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start child services")
	}

	tipHeight, err := dao.blockStore.Height()
	if err != nil {
		return err
	}
	atomic.StoreUint64(&dao.tipHeight, tipHeight)
	return nil
}

func (dao *blockDAO) Stop(ctx context.Context) error { return dao.lifecycle.OnStop(ctx) }

func (dao *blockDAO) Height() (uint64, error) {
	return dao.blockStore.Height()
}

func (dao *blockDAO) GetBlockByHeight(height uint64) (*block.Block, error) {
	return dao.blockStore.GetBlockByHeight(height)
}

func (dao *blockDAO) GetReceipts(height uint64) ([]*action.Receipt, error) {
	return dao.blockStore.GetReceipts(height)
}

func (dao *blockDAO) GetBlockHash(height uint64) (hash.Hash256, error) {
	return dao.blockStore.GetBlockHash(height)
}

func (dao *blockDAO) GetBlockHeight(hash hash.Hash256) (uint64, error) {
	return dao.blockStore.GetBlockHeight(hash)
}

func (dao *blockDAO) GetBlock(hash hash.Hash256) (*block.Block, error) {
	return dao.blockStore.GetBlock(hash)
}

func (dao *blockDAO) HeaderByHeight(height uint64) (*block.Header, error) {
	return dao.blockStore.HeaderByHeight(height)
}

func (dao *blockDAO) FooterByHeight(height uint64) (*block.Footer, error) {
	return dao.blockStore.FooterByHeight(height)
}

func (dao *blockDAO) Header(h hash.Hash256) (*block.Header, error) {
	return dao.blockStore.Header(h)
}

func (dao *blockDAO) ContainsTransactionLog() bool {
	return dao.blockStore.ContainsTransactionLog()
}

func (dao *blockDAO) TransactionLogs(height uint64) (*iotextypes.TransactionLogs, error) {
	return dao.blockStore.TransactionLogs(height)
}

func (dao *blockDAO) PutBlock(ctx context.Context, blk *block.Block) error {
	return dao.blockStore.PutBlock(ctx, blk)
}

func (dao *blockDAO) DeleteTipBlock() error {
	return dao.blockStore.DeleteTipBlock()
}

func (dao *blockDAO) DeleteBlockToTarget(_ uint64) error {
	return nil
}

func (dao *blockDAO) GetActionByActionHash(_ hash.Hash256, _ uint64) (action.SealedEnvelope, uint32, error) {
	return action.SealedEnvelope{}, 0, nil
}

func (dao *blockDAO) GetReceiptByActionHash(_ hash.Hash256, _ uint64) (*action.Receipt, error) {
	return nil, nil
}
