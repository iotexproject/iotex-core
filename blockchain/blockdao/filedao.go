// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/log"
)

const (
	blockHashHeightMappingNS = "h2h"
	systemLogNS              = "syl"
)

var (
	topHeightKey = []byte("th")
	topHashKey   = []byte("ts")
	hashPrefix   = []byte("ha.")
)

// vars
var (
	ErrNotSupported   = errors.New("feature not supported")
	ErrAlreadyExist   = errors.New("block already exist")
	ErrDataCorruption = errors.New("data is corrupted")
)

type (
	// FileDAO represents the data access object for managing block db file
	FileDAO interface {
		Start(ctx context.Context) error
		Stop(ctx context.Context) error
		Height() (uint64, error)
		GetBlockHash(uint64) (hash.Hash256, error)
		GetBlockHeight(hash.Hash256) (uint64, error)
		GetBlock(hash.Hash256) (*block.Block, error)
		GetBlockByHeight(uint64) (*block.Block, error)
		GetReceipts(uint64) ([]*action.Receipt, error)
		ContainsTransactionLog() bool
		TransactionLogs(uint64) (*iotextypes.TransactionLogs, error)
		PutBlock(context.Context, *block.Block) error
		DeleteTipBlock() error
	}

	// fileDAO implements FileDAO
	fileDAO struct {
		legacyFd FileDAO
	}
)

// NewFileDAO creates an instance of FileDAO
func NewFileDAO(compressBlock bool, cfg config.DB) (FileDAO, error) {
	legacyFd, err := newFileDAOLegacy(db.NewBoltDB(cfg), compressBlock, cfg)
	if err != nil {
		return nil, err
	}
	return createFileDAO(legacyFd)
}

// NewFileDAOInMemForTest creates an in-memory FileDAO for testing
func NewFileDAOInMemForTest(cfg config.DB) (FileDAO, error) {
	legacyFd, err := newFileDAOLegacy(db.NewMemKVStore(), false, cfg)
	if err != nil {
		return nil, err
	}
	return createFileDAO(legacyFd)
}

func (fd *fileDAO) Start(ctx context.Context) error {
	return fd.legacyFd.Start(ctx)
}

func (fd *fileDAO) Stop(ctx context.Context) error {
	return fd.legacyFd.Stop(ctx)
}

func (fd *fileDAO) Height() (uint64, error) {
	return fd.legacyFd.Height()
}

func (fd *fileDAO) GetBlockHash(height uint64) (hash.Hash256, error) {
	return fd.legacyFd.GetBlockHash(height)
}

func (fd *fileDAO) GetBlockHeight(hash hash.Hash256) (uint64, error) {
	return fd.legacyFd.GetBlockHeight(hash)
}

func (fd *fileDAO) GetBlock(hash hash.Hash256) (*block.Block, error) {
	return fd.legacyFd.GetBlock(hash)
}

func (fd *fileDAO) GetBlockByHeight(height uint64) (*block.Block, error) {
	return fd.legacyFd.GetBlockByHeight(height)
}

func (fd *fileDAO) GetReceipts(height uint64) ([]*action.Receipt, error) {
	return fd.legacyFd.GetReceipts(height)
}

func (fd *fileDAO) ContainsTransactionLog() bool {
	return fd.legacyFd.ContainsTransactionLog()
}

func (fd *fileDAO) TransactionLogs(height uint64) (*iotextypes.TransactionLogs, error) {
	return fd.legacyFd.TransactionLogs(height)
}

func (fd *fileDAO) PutBlock(ctx context.Context, blk *block.Block) error {
	// bail out if block already exists
	h := blk.HashBlock()
	if _, err := fd.GetBlockHeight(h); err == nil {
		log.L().Error("Block already exists.", zap.Uint64("height", blk.Height()), log.Hex("hash", h[:]))
		return ErrAlreadyExist
	}
	// TODO: check if need to split DB
	return fd.legacyFd.PutBlock(ctx, blk)
}

func (fd *fileDAO) DeleteTipBlock() error {
	return fd.legacyFd.DeleteTipBlock()
}

func createFileDAO(legacy FileDAO) (FileDAO, error) {
	return &fileDAO{
		legacyFd: legacy,
	}, nil
}
