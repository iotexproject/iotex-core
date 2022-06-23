// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package filedao

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/log"
)

const (
	_blockHashHeightMappingNS = "h2h"
	_systemLogNS              = "syl"
)

var (
	_topHeightKey = []byte("th")
	_topHashKey   = []byte("ts")
	_hashPrefix   = []byte("ha.")
)

// vars
var (
	ErrFileNotExist     = errors.New("file does not exist")
	ErrFileCantAccess   = errors.New("cannot access file")
	ErrFileInvalid      = errors.New("file format is not valid")
	ErrNotSupported     = errors.New("feature not supported")
	ErrAlreadyExist     = errors.New("block already exist")
	ErrInvalidTipHeight = errors.New("invalid tip height")
	ErrDataCorruption   = errors.New("data is corrupted")
)

type (
	// BaseFileDAO represents the basic data access object
	BaseFileDAO interface {
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

	// FileDAO represents the data access object for managing block db file
	FileDAO interface {
		BaseFileDAO
		Header(hash.Hash256) (*block.Header, error)
		HeaderByHeight(uint64) (*block.Header, error)
		FooterByHeight(uint64) (*block.Footer, error)
	}

	// fileDAO implements FileDAO
	fileDAO struct {
		lock         sync.Mutex
		topIndex     uint64
		splitHeight  uint64
		cfg          db.Config
		currFd       BaseFileDAO
		legacyFd     FileDAO
		v2Fd         *FileV2Manager // a collection of v2 db files
		deser        *block.Deserializer
		evmNetworkID uint32
	}
)

// NewFileDAO creates an instance of FileDAO
func NewFileDAO(cfg db.Config, evmNetworkID uint32) (FileDAO, error) {
	header, err := checkMasterChainDBFile(cfg.DbPath)
	if err == ErrFileInvalid || err == ErrFileCantAccess {
		return nil, err
	}

	if err == ErrFileNotExist {
		// start new chain db using v2 format
		if err := createNewV2File(1, cfg, evmNetworkID); err != nil {
			return nil, err
		}
		header = &FileHeader{Version: FileV2}
	}

	switch header.Version {
	case FileLegacyMaster:
		// master file is legacy format
		return CreateFileDAO(true, cfg, evmNetworkID)
	case FileV2:
		// master file is v2 format
		return CreateFileDAO(false, cfg, evmNetworkID)
	default:
		panic(fmt.Errorf("corrupted file version: %s", header.Version))
	}
}

// NewFileDAOInMemForTest creates an in-memory FileDAO for testing
func NewFileDAOInMemForTest() (FileDAO, error) {
	return newTestInMemFd()
}

func (fd *fileDAO) Start(ctx context.Context) error {
	if fd.legacyFd != nil {
		if err := fd.legacyFd.Start(ctx); err != nil {
			return err
		}
	}
	if fd.v2Fd != nil {
		if err := fd.v2Fd.Start(ctx); err != nil {
			return err
		}
	}

	if fd.v2Fd != nil {
		fd.currFd, fd.splitHeight = fd.v2Fd.TopFd()
	} else {
		fd.currFd = fd.legacyFd
	}
	return nil
}

func (fd *fileDAO) Stop(ctx context.Context) error {
	if fd.legacyFd != nil {
		if err := fd.legacyFd.Stop(ctx); err != nil {
			return err
		}
	}
	if fd.v2Fd != nil {
		return fd.v2Fd.Stop(ctx)
	}
	return nil
}

func (fd *fileDAO) Height() (uint64, error) {
	return fd.currFd.Height()
}

func (fd *fileDAO) GetBlockHash(height uint64) (hash.Hash256, error) {
	if fd.v2Fd != nil {
		if v2 := fd.v2Fd.FileDAOByHeight(height); v2 != nil {
			return v2.GetBlockHash(height)
		}
	}

	if fd.legacyFd != nil {
		return fd.legacyFd.GetBlockHash(height)
	}
	return hash.ZeroHash256, ErrNotSupported
}

func (fd *fileDAO) GetBlockHeight(hash hash.Hash256) (uint64, error) {
	var (
		height uint64
		err    error
	)
	if fd.v2Fd != nil {
		if height, err = fd.v2Fd.GetBlockHeight(hash); err == nil {
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
	if fd.v2Fd != nil {
		if blk, err = fd.v2Fd.GetBlock(hash); err == nil {
			return blk, nil
		}
	}

	if fd.legacyFd != nil {
		return fd.legacyFd.GetBlock(hash)
	}
	return nil, err
}

func (fd *fileDAO) GetBlockByHeight(height uint64) (*block.Block, error) {
	if fd.v2Fd != nil {
		if v2 := fd.v2Fd.FileDAOByHeight(height); v2 != nil {
			return v2.GetBlockByHeight(height)
		}
	}

	if fd.legacyFd != nil {
		return fd.legacyFd.GetBlockByHeight(height)
	}
	return nil, ErrNotSupported
}

func (fd *fileDAO) Header(hash hash.Hash256) (*block.Header, error) {
	var (
		blk *block.Block
		err error
	)
	if fd.v2Fd != nil {
		if blk, err = fd.v2Fd.GetBlock(hash); err == nil {
			return &blk.Header, nil
		}
	}

	if fd.legacyFd != nil {
		return fd.legacyFd.Header(hash)
	}
	return nil, err
}

func (fd *fileDAO) HeaderByHeight(height uint64) (*block.Header, error) {
	if fd.v2Fd != nil {
		if v2 := fd.v2Fd.FileDAOByHeight(height); v2 != nil {
			blk, err := v2.GetBlockByHeight(height)
			if err != nil {
				return nil, err
			}
			return &blk.Header, nil
		}
	}

	if fd.legacyFd != nil {
		return fd.legacyFd.HeaderByHeight(height)
	}
	return nil, ErrNotSupported
}

func (fd *fileDAO) FooterByHeight(height uint64) (*block.Footer, error) {
	if fd.v2Fd != nil {
		if v2 := fd.v2Fd.FileDAOByHeight(height); v2 != nil {
			blk, err := v2.GetBlockByHeight(height)
			if err != nil {
				return nil, err
			}
			return &blk.Footer, nil
		}
	}

	if fd.legacyFd != nil {
		return fd.legacyFd.FooterByHeight(height)
	}
	return nil, ErrNotSupported
}

func (fd *fileDAO) GetReceipts(height uint64) ([]*action.Receipt, error) {
	if fd.v2Fd != nil {
		if v2 := fd.v2Fd.FileDAOByHeight(height); v2 != nil {
			return v2.GetReceipts(height)
		}
	}

	if fd.legacyFd != nil {
		return fd.legacyFd.GetReceipts(height)
	}
	return nil, ErrNotSupported
}

func (fd *fileDAO) ContainsTransactionLog() bool {
	// TODO: change to ContainsTransactionLog(uint64)
	return fd.currFd.ContainsTransactionLog()
}

func (fd *fileDAO) TransactionLogs(height uint64) (*iotextypes.TransactionLogs, error) {
	if fd.v2Fd != nil {
		if v2 := fd.v2Fd.FileDAOByHeight(height); v2 != nil {
			return v2.TransactionLogs(height)
		}
	}

	if fd.legacyFd != nil {
		return fd.legacyFd.TransactionLogs(height)
	}
	return nil, ErrNotSupported
}

func (fd *fileDAO) PutBlock(ctx context.Context, blk *block.Block) error {
	// bail out if block already exists
	h := blk.HashBlock()
	if _, err := fd.GetBlockHeight(h); err == nil {
		log.L().Error("Block already exists.", zap.Uint64("height", blk.Height()), log.Hex("hash", h[:]))
		return ErrAlreadyExist
	}

	// check if we need to split DB
	if fd.cfg.V2BlocksToSplitDB > 0 {
		if err := fd.prepNextDbFile(blk.Height()); err != nil {
			return err
		}
	}
	return fd.currFd.PutBlock(ctx, blk)
}

func (fd *fileDAO) prepNextDbFile(height uint64) error {
	tip, err := fd.currFd.Height()
	if err != nil {
		return err
	}
	if height != tip+1 {
		return ErrInvalidTipHeight
	}

	fd.lock.Lock()
	defer fd.lock.Unlock()

	if height > fd.splitHeight && height-fd.splitHeight >= fd.cfg.V2BlocksToSplitDB {
		return fd.addNewV2File(height)
	}
	return nil
}

func (fd *fileDAO) addNewV2File(height uint64) error {
	// create a new v2 file
	cfg := fd.cfg
	cfg.DbPath = kthAuxFileName(cfg.DbPath, fd.topIndex+1)
	v2, err := newFileDAOv2(height, cfg, fd.evmNetworkID)
	if err != nil {
		return err
	}

	defer func() {
		if err == nil {
			fd.currFd = v2
			fd.topIndex++
			fd.splitHeight = height
		}
	}()

	// add the new v2 file to existing v2 manager
	ctx := context.Background()
	if fd.v2Fd != nil {
		if err = fd.v2Fd.AddFileDAO(v2, height); err != nil {
			return err
		}
		err = v2.Start(ctx)
		return err
	}

	// create v2 manager
	fd.v2Fd, _ = newFileV2Manager([]*fileDAOv2{v2})
	err = fd.v2Fd.Start(ctx)
	return err
}

func (fd *fileDAO) DeleteTipBlock() error {
	return fd.currFd.DeleteTipBlock()
}

// CreateFileDAO creates FileDAO according to master file
func CreateFileDAO(legacy bool, cfg db.Config, evmNetworkID uint32) (FileDAO, error) {
	fd := fileDAO{splitHeight: 1, cfg: cfg, evmNetworkID: evmNetworkID}
	fds := []*fileDAOv2{}
	v2Top, v2Files := checkAuxFiles(cfg.DbPath, FileV2)
	if legacy {
		legacyFd, err := newFileDAOLegacy(cfg, evmNetworkID)
		if err != nil {
			return nil, err
		}
		fd.legacyFd = legacyFd
		fd.topIndex, _ = checkAuxFiles(cfg.DbPath, FileLegacyAuxiliary)

		// legacy master file with no v2 files, early exit
		if len(v2Files) == 0 {
			return &fd, nil
		}
	} else {
		// v2 master file
		fds = append(fds, openFileDAOv2(cfg, evmNetworkID))
	}

	// populate v2 files into v2 manager
	if len(v2Files) > 0 {
		for _, name := range v2Files {
			cfg.DbPath = name
			fds = append(fds, openFileDAOv2(cfg, evmNetworkID))
		}

		// v2 file's top index overrides v1's top
		fd.topIndex = v2Top
	}
	v2Fd, err := newFileV2Manager(fds)
	if err != nil {
		return nil, err
	}
	fd.v2Fd = v2Fd
	return &fd, nil
}

// createNewV2File creates a new v2 chain db file
func createNewV2File(start uint64, cfg db.Config, evmNetworkID uint32) error {
	v2, err := newFileDAOv2(start, cfg, evmNetworkID)
	if err != nil {
		return err
	}

	// calling Start() will write the header
	ctx := context.Background()
	if err := v2.Start(ctx); err != nil {
		return err
	}
	return v2.Stop(ctx)
}
