// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"github.com/iotexproject/iotex-core/common/service"
	"github.com/iotexproject/iotex-core/common/utils"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/db"
	"github.com/pkg/errors"
)

const (
	blockNS                  = "blocks"
	blockHashHeightMappingNS = "hash<->height"
)

var (
	hashPrefix   = []byte("hash.")
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
	err = dao.kvstore.PutIfNotExists(blockNS, topHeightKey, make([]byte, 4))
	if err != nil {
		return errors.Wrap(err, "failed to write initial value for top height")
	}
	return nil
}

// getBlockHash returns the block hash by height
func (dao *blockDAO) getBlockHash(height uint32) (crypto.Hash32B, error) {
	key := append(heightPrefix, utils.Uint32ToBytes(height)...)
	value, err := dao.kvstore.Get(blockHashHeightMappingNS, key)
	var hash crypto.Hash32B
	if err != nil {
		return hash, errors.Wrap(err, "failed to get block hash")
	}
	copy(hash[:], value)
	return hash, nil
}

// getBlockHeight returns the block height by hash
func (dao *blockDAO) getBlockHeight(hash crypto.Hash32B) (uint32, error) {
	key := append(hashPrefix, hash[:]...)
	value, err := dao.kvstore.Get(blockHashHeightMappingNS, key)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get block height")
	}
	return utils.BytesToUint32(value), nil
}

// getBlock returns a block
func (dao *blockDAO) getBlock(hash crypto.Hash32B) (*Block, error) {
	value, err := dao.kvstore.Get(blockNS, hash[:])
	if err != nil {
		return nil, errors.Wrap(err, "failed to get block")
	}
	blk := Block{}
	if err = blk.Deserialize(value); err != nil {
		return nil, errors.Wrap(err, "failed to deserialize block")
	}
	return &blk, nil
}

// getBlockchainHeight returns the blockchain height
func (dao *blockDAO) getBlockchainHeight() (uint32, error) {
	value, err := dao.kvstore.Get(blockNS, topHeightKey)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get top height")
	}
	return utils.BytesToUint32(value), nil
}

// putBlock puts a block
func (dao *blockDAO) putBlock(blk *Block) error {
	hash := blk.HashBlock()
	height := utils.Uint32ToBytes(blk.Height())
	serialized, err := blk.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize block")
	}
	if err = dao.kvstore.Put(blockNS, hash[:], serialized); err != nil {
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
	topHeight := utils.BytesToUint32(value)
	if blk.Height() > topHeight {
		dao.kvstore.Put(blockNS, topHeightKey, height)
		if err != nil {
			return errors.Wrap(err, "failed to get top height")
		}
	}
	return nil
}
