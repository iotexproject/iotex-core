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
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
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
	ErrNotSupported     = errors.New("feature not supported")
	ErrAlreadyExist     = errors.New("block already exist")
	ErrInvalidTipHeight = errors.New("invalid tip height")
	ErrDataCorruption   = errors.New("data is corrupted")
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
		lifecycle lifecycle.Lifecycle
		currFd    FileDAO
		legacyFd  FileDAO
		v2Fd      map[uint64]FileDAO
	}
)

// NewFileDAO creates an instance of FileDAO
func NewFileDAO(compressBlock bool, cfg config.DB) (FileDAO, error) {
	legacyFd, err := newFileDAOLegacy(db.NewBoltDB(cfg), compressBlock, cfg)
	if err != nil {
		return nil, err
	}
	return createFileDAO(legacyFd, nil)
}

// NewFileDAOInMemForTest creates an in-memory FileDAO for testing
func NewFileDAOInMemForTest(cfg config.DB) (FileDAO, error) {
	legacyFd, err := newFileDAOLegacy(db.NewMemKVStore(), false, cfg)
	if err != nil {
		return nil, err
	}
	return createFileDAO(legacyFd, nil)
}

func (fd *fileDAO) Start(ctx context.Context) error {
	return fd.lifecycle.OnStart(ctx)
}

func (fd *fileDAO) Stop(ctx context.Context) error {
	return fd.lifecycle.OnStop(ctx)
}

func (fd *fileDAO) Height() (uint64, error) {
	return fd.currFd.Height()
}

func (fd *fileDAO) GetBlockHash(height uint64) (hash.Hash256, error) {
	var (
		h   hash.Hash256
		err error
	)
	for _, file := range fd.v2Fd {
		if h, err = file.GetBlockHash(height); err == nil {
			return h, nil
		}
	}
	if fd.legacyFd != nil {
		return fd.legacyFd.GetBlockHash(height)
	}
	return hash.ZeroHash256, err
}

func (fd *fileDAO) GetBlockHeight(hash hash.Hash256) (uint64, error) {
	var (
		height uint64
		err    error
	)
	for _, file := range fd.v2Fd {
		if height, err = file.GetBlockHeight(hash); err == nil {
			return height, nil
		}
	}
	if fd.legacyFd != nil {
		return fd.legacyFd.GetBlockHeight(hash)
	}
	return 0, err
}

func (fd *fileDAO) GetBlock(hash hash.Hash256) (*block.Block, error) {
	var (
		blk *block.Block
		err error
	)
	for _, file := range fd.v2Fd {
		if blk, err = file.GetBlock(hash); err == nil {
			return blk, nil
		}
	}
	if fd.legacyFd != nil {
		return fd.legacyFd.GetBlock(hash)
	}
	return nil, err
}

func (fd *fileDAO) GetBlockByHeight(height uint64) (*block.Block, error) {
	var (
		blk *block.Block
		err error
	)
	for _, file := range fd.v2Fd {
		if blk, err = file.GetBlockByHeight(height); err == nil {
			return blk, nil
		}
	}
	if fd.legacyFd != nil {
		return fd.legacyFd.GetBlockByHeight(height)
	}
	return nil, err
}

func (fd *fileDAO) GetReceipts(height uint64) ([]*action.Receipt, error) {
	var (
		receipts []*action.Receipt
		err      error
	)
	for _, file := range fd.v2Fd {
		if receipts, err = file.GetReceipts(height); err == nil {
			return receipts, nil
		}
	}
	if fd.legacyFd != nil {
		return fd.legacyFd.GetReceipts(height)
	}
	return nil, err
}

func (fd *fileDAO) ContainsTransactionLog() bool {
	// TODO: change to ContainsTransactionLog(uint64)
	return fd.currFd.ContainsTransactionLog()
}

func (fd *fileDAO) TransactionLogs(height uint64) (*iotextypes.TransactionLogs, error) {
	var (
		log *iotextypes.TransactionLogs
		err error
	)
	for _, file := range fd.v2Fd {
		if log, err = file.TransactionLogs(height); err == nil {
			return log, nil
		}
	}
	if fd.legacyFd != nil {
		return fd.legacyFd.TransactionLogs(height)
	}
	return nil, err
}

func (fd *fileDAO) PutBlock(ctx context.Context, blk *block.Block) error {
	// bail out if block already exists
	h := blk.HashBlock()
	if _, err := fd.GetBlockHeight(h); err == nil {
		log.L().Error("Block already exists.", zap.Uint64("height", blk.Height()), log.Hex("hash", h[:]))
		return ErrAlreadyExist
	}
	// TODO: check if need to split DB
	return fd.currFd.PutBlock(ctx, blk)
}

func (fd *fileDAO) DeleteTipBlock() error {
	return fd.currFd.DeleteTipBlock()
}

func createFileDAO(legacy FileDAO, newFile map[uint64]FileDAO) (FileDAO, error) {
	fileDAO := &fileDAO{
		legacyFd: legacy,
		v2Fd:     newFile,
	}

	var (
		tipHeight uint64
		currFd    FileDAO
	)

	// find the file with highest start height
	for start, fd := range newFile {
		if currFd == nil {
			currFd = fd
			tipHeight = start
		} else {
			if start > tipHeight {
				currFd = fd
				tipHeight = start
			}
		}
		fileDAO.lifecycle.Add(fd)
	}
	if legacy != nil {
		fileDAO.lifecycle.Add(legacy)
		if currFd == nil {
			currFd = legacy
		}
	}

	if currFd == nil {
		return nil, errors.New("failed to find valid chain db file")
	}
	fileDAO.currFd = currFd
	return fileDAO, nil
}
