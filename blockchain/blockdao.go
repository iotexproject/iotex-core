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
	blockNS                            = "blocks"
	blockHashHeightMappingNS           = "hash<->height"
	blockTransferBlockMappingNS        = "transfer<->block"
	blockAddressTransferMappingNS      = "address<->transfer"
	blockAddressTransferCountMappingNS = "address<->transfercount"
)

var (
	hashPrefix         = []byte("hash.")
	transferPrefix     = []byte("transfer.")
	heightPrefix       = []byte("height.")
	topHeightKey       = []byte("top-height")
	transferFromPrefix = []byte("transfer-from.")
	transferToPrefix   = []byte("transfer-to.")
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

func (dao *blockDAO) getBlockHashByTransferHash(hash common.Hash32B) (common.Hash32B, error) {
	blkHash := common.ZeroHash32B
	key := append(transferPrefix, hash[:]...)
	value, err := dao.kvstore.Get(blockTransferBlockMappingNS, key)
	if err != nil {
		return blkHash, errors.Wrapf(err, "failed to get transfer %x", hash)
	}
	if len(value) == 0 {
		return blkHash, errors.Wrapf(db.ErrNotExist, "transfer %x missing", hash)
	}
	copy(blkHash[:], value)
	return blkHash, nil
}

func (dao *blockDAO) getTransfersBySenderAddress(address string) ([]common.Hash32B, error) {
	// get transfers count for sender
	senderTransferCount, err := dao.getTransferCountBySenderAddress(address)
	if err != nil {
		return nil, errors.Wrapf(err, "for sender %x", address)
	}

	res, getTransfersErr := dao.getTransfersByAddress(address, senderTransferCount, transferFromPrefix)
	if getTransfersErr != nil {
		return nil, getTransfersErr
	}

	return res, nil
}

func (dao *blockDAO) getTransferCountBySenderAddress(address string) (uint64, error) {
	senderTransferCountKey := append(transferFromPrefix, address...)
	value, err := dao.kvstore.Get(blockAddressTransferCountMappingNS, senderTransferCountKey)
	if err != nil {
		return 0, nil
	}
	if len(value) == 0 {
		return 0, errors.New("count of transfers as recipient is broken")
	}
	return common.MachineEndian.Uint64(value), nil
}

func (dao *blockDAO) getTransfersByRecipientAddress(address string) ([]common.Hash32B, error) {
	// get transfers count for recipient
	recipientTransferCount, getCountErr := dao.getTransferCountByRecipientAddress(address)
	if getCountErr != nil {
		return nil, errors.Wrapf(getCountErr, "for recipient %x", address)
	}

	res, getTransfersErr := dao.getTransfersByAddress(address, recipientTransferCount, transferToPrefix)
	if getTransfersErr != nil {
		return nil, getTransfersErr
	}

	return res, nil
}

func (dao *blockDAO) getTransfersByAddress(address string, count uint64, keyPrefix []byte) ([]common.Hash32B, error) {
	var res []common.Hash32B

	for i := uint64(0); i < count; i++ {
		// put new transfer to recipient
		key := append(keyPrefix, address...)
		key = append(key, utils.Uint64ToBytes(i)...)
		value, err := dao.kvstore.Get(blockAddressTransferMappingNS, key)
		if err != nil {
			return res, errors.Wrapf(err, "failed to get transfer for index %x", i)
		}
		if len(value) == 0 {
			return res, errors.Wrapf(db.ErrNotExist, "transfer for index %x missing", i)
		}
		transferHash := common.ZeroHash32B
		copy(transferHash[:], value)
		res = append(res, transferHash)
	}

	return res, nil
}

func (dao *blockDAO) getTransferCountByRecipientAddress(address string) (uint64, error) {
	recipientTransferCountKey := append(transferToPrefix, address...)
	value, err := dao.kvstore.Get(blockAddressTransferCountMappingNS, recipientTransferCountKey)
	if err != nil {
		return 0, nil
	}
	if len(value) == 0 {
		return 0, errors.New("count of transfers as recipient is broken")
	}
	return common.MachineEndian.Uint64(value), nil
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
	// map Transfer hash to block hash
	for _, transfer := range blk.Transfers {
		transferHash := transfer.Hash()
		hashKey := append(transferPrefix, transferHash[:]...)
		if err = dao.kvstore.Put(blockTransferBlockMappingNS, hashKey, hash[:]); err != nil {
			return errors.Wrapf(err, "failed to put transfer hash %x", transferHash)
		}
	}

	for _, transfer := range blk.Transfers {
		transferHash := transfer.Hash()

		// get transfers count for sender
		senderTransferCount, err := dao.getTransferCountBySenderAddress(transfer.Sender)
		if err != nil {
			return errors.Wrapf(err, "for sender %x", transfer.Sender)
		}

		// put new transfer to sender
		senderKey := append(transferFromPrefix, transfer.Sender...)
		senderKey = append(senderKey, utils.Uint64ToBytes(senderTransferCount)...)
		if err = dao.kvstore.PutIfNotExists(blockAddressTransferMappingNS, senderKey, transferHash[:]); err != nil {
			return errors.Wrapf(err, "failed to put transfer hash %x for sender %x",
				transfer.Hash(), transfer.Sender)
		}

		// update sender transfers count
		senderTransferCountKey := append(transferFromPrefix, transfer.Sender...)
		if err = dao.kvstore.Put(blockAddressTransferCountMappingNS, senderTransferCountKey,
			utils.Uint64ToBytes(senderTransferCount+1)); err != nil {
			return errors.Wrapf(err, "failed to bump transfer count %x for sender %x",
				transfer.Hash(), transfer.Sender)
		}

		// get transfers count for recipient
		recipientTransferCount, err := dao.getTransferCountByRecipientAddress(transfer.Recipient)
		if err != nil {
			return errors.Wrapf(err, "for recipient %x", transfer.Recipient)
		}

		// put new transfer to recipient
		recipientKey := append(transferToPrefix, transfer.Recipient...)
		recipientKey = append(recipientKey, utils.Uint64ToBytes(recipientTransferCount)...)
		if err = dao.kvstore.PutIfNotExists(blockAddressTransferMappingNS, recipientKey, transferHash[:]); err != nil {
			return errors.Wrapf(err, "failed to put transfer hash %x for recipient %x",
				transfer.Hash(), transfer.Recipient)
		}

		// update recipient transfers count
		recipientTransferCountKey := append(transferToPrefix, transfer.Recipient...)
		if err = dao.kvstore.Put(blockAddressTransferCountMappingNS, recipientTransferCountKey,
			utils.Uint64ToBytes(recipientTransferCount+1)); err != nil {
			return errors.Wrapf(err, "failed to bump transfer count %x for recipient %x",
				transfer.Hash(), transfer.Recipient)
		}
	}

	return nil
}
