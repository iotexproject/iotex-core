// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"encoding/hex"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"

	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/pkg/errors"
)

// FileBasedChain defines a Blockchain db based on file system
type FileBasedChain struct {
	chainDir  string
	lifecycle lifecycle.Lifecycle
}

// NewFileBasedChain returns
func NewFileBasedChain(chainDir string) *FileBasedChain {
	stat, err := os.Stat(chainDir)
	// create the chain dir if not exist
	if err != nil {
		if os.ErrNotExist != err {
			logger.Error().Err(err).Msg("failed to create blockchain")
			return nil
		}
		if err := os.MkdirAll(chainDir, os.ModePerm); err != nil {
			logger.Error().Err(err).Msg("failed to create blockchain")
		}
	}
	if !stat.IsDir() {
		logger.Error().Str("chainDir", chainDir).Msg("Chain path is not a directory")
		return nil
	}
	return &FileBasedChain{chainDir: chainDir}
}

// Start starts the blockchain
func (c *FileBasedChain) Start(ctx context.Context) error {
	if err := c.lifecycle.OnStart(ctx); err != nil {
		return err
	}
	return nil
}

// Stop stops the blockchain
func (c *FileBasedChain) Stop(ctx context.Context) error {
	return c.lifecycle.OnStop(ctx)
}

// Height returns the chain height
// Warning: this is a high-cost execution, which will loop all files stored in chainDir
func (c *FileBasedChain) Height() (uint64, error) {
	var height uint64
	err := filepath.Walk(c.chainDir, func(path string, info os.FileInfo, err error) error {
		if hexNum, err := hex.DecodeString(info.Name()); err != nil {
			num := byteutil.BytesToUint64(hexNum)
			if num > height {
				height = num
			}
		}
		return nil
	})
	if err != nil {
		return 0, errors.Wrap(err, "failed to get top height")
	}
	return height, nil
}

// BlockByHeight returns the block of height
func (c *FileBasedChain) BlockByHeight(h uint64) (*Block, error) {
	data, err := ioutil.ReadFile(c.blockPath(h))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read block file")
	}
	blk := Block{}
	if err = blk.Deserialize(data); err != nil {
		return nil, errors.Wrap(err, "failed to deserialize block")
	}

	return &blk, nil
}

// PutBlock writes block of height to file
func (c *FileBasedChain) PutBlock(h uint64, blk *Block) error {
	// TODO: overwrite checking
	data, err := blk.Serialize()
	if err != nil {
		return errors.Wrapf(err, "failed to serialize block")
	}
	err = ioutil.WriteFile(c.blockPath(h), data, os.ModePerm)
	if err != nil {
		return errors.Wrapf(err, "failed to write block file")
	}

	return nil
}

func (c *FileBasedChain) blockPath(h uint64) string {
	return path.Join(c.chainDir, hex.EncodeToString(byteutil.Uint64ToBytes(h)))
}
