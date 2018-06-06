// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/common/service"
	"github.com/iotexproject/iotex-core/common/utils"
	"github.com/iotexproject/iotex-core/db"
)

const (
	blockNS                  = "blocks"
	blockHashHeightMappingNS = "hash<->height"
	blockTxBlockMappingNS    = "tx<->block"
)

var (
	hashPrefix   = []byte("hash.")
	txPrefix     = []byte("tx.")
	heightPrefix = []byte("height.")
	topHeightKey = []byte("top-height")
)

type blockDAO struct {
	service.CompositeService
	kvstore db.KVStore
}

// newBlockDAO instantiates a block DAO
func newBlockDAO(kvstore db.KVStore) *blockDAO {
	blockDAO := &blockDAO{kvstore: kvstore}
	blockDAO.AddService(kvstore)
	return blockDAO
}

// Start starts block DAO and initiates the top height if it doesn't exist
func (dao *blockDAO) Start() error {
	err := dao.CompositeService.Start()
	if err != nil {
		return errors.Wrap(err, "failed to start child services")
	}

	// set init height value
	err = dao.kvstore.PutIfNotExists(blockNS, topHeightKey, make([]byte, 8))
	if err != nil {
		return errors.Wrap(err, "failed to write initial value for top height")
	}
	return nil
}

// getBlockHash returns the block hash by height
func (dao *blockDAO) getBlockHash(height uint64) (common.Hash32B, error) {
	key := append(heightPrefix, utils.Uint64ToBytes(height)...)
	value, err := dao.kvstore.Get(blockHashHeightMappingNS, key)
	hash := common.ZeroHash32B
	if err != nil {
		return hash, errors.Wrap(err, "failed to get block hash")
	}
	copy(hash[:], value)
	return hash, nil
}

// getBlockHeight returns the block height by hash
func (dao *blockDAO) getBlockHeight(hash common.Hash32B) (uint64, error) {
	key := append(hashPrefix, hash[:]...)
	value, err := dao.kvstore.Get(blockHashHeightMappingNS, key)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get block height")
	}
	if value == nil || len(value) == 0 {
		return 0, errors.Wrapf(db.ErrNotExist, "height missing for block with hash = %x", hash)
	}
	return common.MachineEndian.Uint64(value), nil
}

// getBlock returns a block
func (dao *blockDAO) getBlock(hash common.Hash32B) (*Block, error) {
	value, err := dao.kvstore.Get(blockNS, hash[:])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block %x", hash)
	}
	if len(value) == 0 {
		return nil, errors.Wrapf(db.ErrNotExist, "block %x missing", hash)
	}
	blk := Block{}
	if err = blk.Deserialize(value); err != nil {
		return nil, errors.Wrap(err, "failed to deserialize block")
	}
	return &blk, nil
}

func (dao *blockDAO) getBlockHashByTxHash(hash common.Hash32B) (common.Hash32B, error) {
	blkHash := common.ZeroHash32B
	key := append(txPrefix, hash[:]...)
	value, err := dao.kvstore.Get(blockTxBlockMappingNS, key)
	if err != nil {
		return blkHash, errors.Wrapf(err, "failed to get tx %x", hash)
	}
	if len(value) == 0 {
		return blkHash, errors.Wrapf(db.ErrNotExist, "tx %x missing", hash)
	}
	copy(blkHash[:], value)
	return blkHash, nil
}

// getBlockchainHeight returns the blockchain height
func (dao *blockDAO) getBlockchainHeight() (uint64, error) {
	value, err := dao.kvstore.Get(blockNS, topHeightKey)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get top height")
	}
	if value == nil || len(value) == 0 {
		return 0, errors.Wrap(db.ErrNotExist, "blockchain height missing")
	}
	return common.MachineEndian.Uint64(value), nil
}

// putBlock puts a block
func (dao *blockDAO) putBlock(blk *Block) error {
	height := utils.Uint64ToBytes(blk.Height())
	serialized, err := blk.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize block")
	}
	hash := blk.HashBlock()
	if err = dao.kvstore.PutIfNotExists(blockNS, hash[:], serialized); err != nil {
		return errors.Wrap(err, "failed to put block")
	}
	hashKey := append(hashPrefix, hash[:]...)
	if err = dao.kvstore.Put(blockHashHeightMappingNS, hashKey, height); err != nil {
		return errors.Wrap(err, "failed to put hash -> height mapping")
	}
	heightKey := append(heightPrefix, height...)
	if err = dao.kvstore.Put(blockHashHeightMappingNS, heightKey, hash[:]); err != nil {
		return errors.Wrap(err, "failed to put height -> hash mapping")
	}
	value, err := dao.kvstore.Get(blockNS, topHeightKey)
	if err != nil {
		return errors.Wrap(err, "failed to get top height")
	}
	topHeight := common.MachineEndian.Uint64(value)
	if blk.Height() > topHeight {
		if err = dao.kvstore.Put(blockNS, topHeightKey, height); err != nil {
			return errors.Wrap(err, "failed to put top height")
		}
	}
	// map Tx hash to block hash
	for _, tx := range blk.Tranxs {
		txHash := tx.Hash()
		hashKey := append(txPrefix, txHash[:]...)
		if err = dao.kvstore.Put(blockTxBlockMappingNS, hashKey, hash[:]); err != nil {
			return errors.Wrapf(err, "failed to put tx hash %x", txHash)
		}
	}
	return nil
}
