// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/proto"
)

const (
	blockNS                             = "blocks"
	blockHashHeightMappingNS            = "hash<->height"
	blockTransferBlockMappingNS         = "transfer<->block"
	blockVoteBlockMappingNS             = "vote<->block"
	blockExecutionBlockMappingNS        = "execution<->block"
	blockActionBlockMappingNS           = "action<->block"
	blockActionReceiptMappingNS         = "action<->receipt"
	blockAddressTransferMappingNS       = "address<->transfer"
	blockAddressTransferCountMappingNS  = "address<->transfercount"
	blockAddressVoteMappingNS           = "address<->vote"
	blockAddressVoteCountMappingNS      = "address<->votecount"
	blockAddressExecutionMappingNS      = "address<->execution"
	blockAddressExecutionCountMappingNS = "address<->executioncount"
	blockAddressActionMappingNS         = "address<->action"
	blockAddressActionCountMappingNS    = "address<->actioncount"
	receiptsNS                          = "receipts"
)

var (
	hashPrefix      = []byte("hash.")
	transferPrefix  = []byte("transfer.")
	votePrefix      = []byte("vote.")
	executionPrefix = []byte("execution.")
	actionPrefix    = []byte("action.")
	heightPrefix    = []byte("height.")
	// mutate this field is not thread safe, pls only mutate it in putBlock!
	topHeightKey = []byte("top-height")
	// mutate this field is not thread safe, pls only mutate it in putBlock!
	totalTransfersKey   = []byte("total-transfers")
	totalVotesKey       = []byte("total-votes")
	totalExecutionsKey  = []byte("total-executions")
	totalActionsKey     = []byte("total-actions")
	transferFromPrefix  = []byte("transfer-from.")
	transferToPrefix    = []byte("transfer-to.")
	voteFromPrefix      = []byte("vote-from.")
	voteToPrefix        = []byte("vote-to.")
	executionFromPrefix = []byte("execution-from")
	executionToPrefix   = []byte("execution-to")
	actionFromPrefix    = []byte("action-from")
	actionToPrefix      = []byte("action-to")
)

var _ lifecycle.StartStopper = (*blockDAO)(nil)

type blockDAO struct {
	writeIndex bool
	kvstore    db.KVStore
	lifecycle  lifecycle.Lifecycle
}

// newBlockDAO instantiates a block DAO
func newBlockDAO(kvstore db.KVStore, writeIndex bool) *blockDAO {
	blockDAO := &blockDAO{
		writeIndex: writeIndex,
		kvstore:    kvstore,
	}
	blockDAO.lifecycle.Add(kvstore)
	return blockDAO
}

// Start starts block DAO and initiates the top height if it doesn't exist
func (dao *blockDAO) Start(ctx context.Context) error {
	err := dao.lifecycle.OnStart(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start child services")
	}

	// set init height value
	// TODO: not working with badger, we shouldn't expose detailed db error (e.g., bolt.ErrBucketExists) to application
	if _, err = dao.kvstore.Get(blockNS, topHeightKey); err != nil &&
		(errors.Cause(err) == db.ErrNotExist || errors.Cause(err) == bolt.ErrBucketNotFound) {
		if err := dao.kvstore.Put(blockNS, topHeightKey, make([]byte, 8)); err != nil {
			return errors.Wrap(err, "failed to write initial value for top height")
		}
	}

	// TODO: To be deprecated
	// set init total transfer to be 0
	if _, err := dao.kvstore.Get(blockNS, totalTransfersKey); err != nil &&
		(errors.Cause(err) == db.ErrNotExist || errors.Cause(err) == bolt.ErrBucketNotFound) {
		if err = dao.kvstore.Put(blockNS, totalTransfersKey, make([]byte, 8)); err != nil {
			return errors.Wrap(err, "failed to write initial value for total transfers")
		}
	}

	// TODO: To be deprecated
	// set init total vote to be 0
	if _, err := dao.kvstore.Get(blockNS, totalVotesKey); err != nil &&
		(errors.Cause(err) == db.ErrNotExist || errors.Cause(err) == bolt.ErrBucketNotFound) {
		if err = dao.kvstore.Put(blockNS, totalVotesKey, make([]byte, 8)); err != nil {
			return errors.Wrap(err, "failed to write initial value for total votes")
		}
	}

	// TODO: To be deprecated
	// set init total executions to be 0
	if _, err := dao.kvstore.Get(blockNS, totalExecutionsKey); err != nil &&
		(errors.Cause(err) == db.ErrNotExist || errors.Cause(err) == bolt.ErrBucketNotFound) {
		if err = dao.kvstore.Put(blockNS, totalExecutionsKey, make([]byte, 8)); err != nil {
			return errors.Wrap(err, "failed to write initial value for total executions")
		}
	}

	// set init total actions to be 0
	if _, err := dao.kvstore.Get(blockNS, totalActionsKey); err != nil &&
		(errors.Cause(err) == db.ErrNotExist || errors.Cause(err) == bolt.ErrBucketNotFound) {
		if err = dao.kvstore.Put(blockNS, totalActionsKey, make([]byte, 8)); err != nil {
			return errors.Wrap(err, "failed to write initial value for total actions")
		}
	}

	return nil
}

// Stop stops block DAO.
func (dao *blockDAO) Stop(ctx context.Context) error { return dao.lifecycle.OnStop(ctx) }

// getBlockHash returns the block hash by height
func (dao *blockDAO) getBlockHash(height uint64) (hash.Hash32B, error) {
	key := append(heightPrefix, byteutil.Uint64ToBytes(height)...)
	value, err := dao.kvstore.Get(blockHashHeightMappingNS, key)
	hash := hash.ZeroHash32B
	if err != nil {
		return hash, errors.Wrap(err, "failed to get block hash")
	}
	if len(hash) != len(value) {
		return hash, errors.Wrap(err, "blockhash is broken")
	}
	copy(hash[:], value)
	return hash, nil
}

// getBlockHeight returns the block height by hash
func (dao *blockDAO) getBlockHeight(hash hash.Hash32B) (uint64, error) {
	key := append(hashPrefix, hash[:]...)
	value, err := dao.kvstore.Get(blockHashHeightMappingNS, key)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get block height")
	}
	if len(value) == 0 {
		return 0, errors.Wrapf(db.ErrNotExist, "height missing for block with hash = %x", hash)
	}
	return enc.MachineEndian.Uint64(value), nil
}

// getBlock returns a block
func (dao *blockDAO) getBlock(hash hash.Hash32B) (*block.Block, error) {
	value, err := dao.kvstore.Get(blockNS, hash[:])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block %x", hash)
	}
	if len(value) == 0 {
		return nil, errors.Wrapf(db.ErrNotExist, "block %x missing", hash)
	}
	blk := block.Block{}
	if err = blk.Deserialize(value); err != nil {
		return nil, errors.Wrap(err, "failed to deserialize block")
	}
	return &blk, nil
}

// TODO: To be deprecated
func (dao *blockDAO) getBlockHashByTransferHash(h hash.Hash32B) (hash.Hash32B, error) {
	blkHash := hash.ZeroHash32B
	key := append(transferPrefix, h[:]...)
	value, err := dao.kvstore.Get(blockTransferBlockMappingNS, key)
	if err != nil {
		return blkHash, errors.Wrapf(err, "failed to get transfer %x", h)
	}
	if len(value) == 0 {
		return blkHash, errors.Wrapf(db.ErrNotExist, "transfer %x missing", h)
	}
	copy(blkHash[:], value)
	return blkHash, nil
}

// TODO: To be deprecated
func (dao *blockDAO) getBlockHashByVoteHash(h hash.Hash32B) (hash.Hash32B, error) {
	blkHash := hash.ZeroHash32B
	key := append(votePrefix, h[:]...)
	value, err := dao.kvstore.Get(blockVoteBlockMappingNS, key)
	if err != nil {
		return blkHash, errors.Wrapf(err, "failed to get vote %x", h)
	}
	if len(value) == 0 {
		return blkHash, errors.Wrapf(db.ErrNotExist, "vote %x missing", h)
	}
	copy(blkHash[:], value)
	return blkHash, nil
}

// TODO: To be deprecated
func (dao *blockDAO) getBlockHashByExecutionHash(h hash.Hash32B) (hash.Hash32B, error) {
	blkHash := hash.ZeroHash32B
	key := append(executionPrefix, h[:]...)
	value, err := dao.kvstore.Get(blockExecutionBlockMappingNS, key)
	if err != nil {
		return blkHash, errors.Wrapf(err, "failed to get execution %x", h)
	}
	if len(value) == 0 {
		return blkHash, errors.Wrapf(db.ErrNotExist, "execution %x missing", h)
	}
	copy(blkHash[:], value)
	return blkHash, nil
}

func (dao *blockDAO) getBlockHashByActionHash(h hash.Hash32B) (hash.Hash32B, error) {
	blkHash := hash.ZeroHash32B
	key := append(actionPrefix, h[:]...)
	value, err := dao.kvstore.Get(blockActionBlockMappingNS, key)
	if err != nil {
		return blkHash, errors.Wrapf(err, "failed to get action %x", h)
	}
	if len(value) == 0 {
		return blkHash, errors.Wrapf(db.ErrNotExist, "action %x missing", h)
	}
	copy(blkHash[:], value)
	return blkHash, nil
}

// TODO: To be deprecated
// getTransfersBySenderAddress returns transfers for sender
func (dao *blockDAO) getTransfersBySenderAddress(address string) ([]hash.Hash32B, error) {
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

// TODO: To be deprecated
// getTransferCountBySenderAddress returns transfer count by sender address
func (dao *blockDAO) getTransferCountBySenderAddress(address string) (uint64, error) {
	senderTransferCountKey := append(transferFromPrefix, address...)
	value, err := dao.kvstore.Get(blockAddressTransferCountMappingNS, senderTransferCountKey)
	if err != nil {
		return 0, nil
	}
	if len(value) == 0 {
		return 0, errors.New("count of transfers as recipient is broken")
	}
	return enc.MachineEndian.Uint64(value), nil
}

// TODO: To be deprecated
// getTransfersByRecipientAddress returns transfers for recipient
func (dao *blockDAO) getTransfersByRecipientAddress(address string) ([]hash.Hash32B, error) {
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

// TODO: To be deprecated
// getTransfersByAddress returns transfers by address
func (dao *blockDAO) getTransfersByAddress(address string, count uint64, keyPrefix []byte) ([]hash.Hash32B, error) {
	var res []hash.Hash32B

	for i := uint64(0); i < count; i++ {
		// put new transfer to recipient
		key := append(keyPrefix, address...)
		key = append(key, byteutil.Uint64ToBytes(i)...)
		value, err := dao.kvstore.Get(blockAddressTransferMappingNS, key)
		if err != nil {
			return res, errors.Wrapf(err, "failed to get transfer for index %x", i)
		}
		if len(value) == 0 {
			return res, errors.Wrapf(db.ErrNotExist, "transfer for index %x missing", i)
		}
		transferHash := hash.ZeroHash32B
		copy(transferHash[:], value)
		res = append(res, transferHash)
	}

	return res, nil
}

// TODO: To be deprecated
// getTransferCountByRecipientAddress returns transfer count by recipient address
func (dao *blockDAO) getTransferCountByRecipientAddress(address string) (uint64, error) {
	recipientTransferCountKey := append(transferToPrefix, address...)
	value, err := dao.kvstore.Get(blockAddressTransferCountMappingNS, recipientTransferCountKey)
	if err != nil {
		return 0, nil
	}
	if len(value) == 0 {
		return 0, errors.New("count of transfers as recipient is broken")
	}
	return enc.MachineEndian.Uint64(value), nil
}

// TODO: To be deprecated
// getVotesBySenderAddress returns votes for sender
func (dao *blockDAO) getVotesBySenderAddress(address string) ([]hash.Hash32B, error) {
	senderVoteCount, err := dao.getVoteCountBySenderAddress(address)
	if err != nil {
		return nil, errors.Wrapf(err, "to get votecount for sender %x", address)
	}

	res, err := dao.getVotesByAddress(address, senderVoteCount, voteFromPrefix)
	if err != nil {
		return nil, errors.Wrapf(err, "to get votes for sender %x", address)
	}

	return res, nil
}

// TODO: To be deprecated
// getVoteCountBySenderAddress returns vote count by sender address
func (dao *blockDAO) getVoteCountBySenderAddress(address string) (uint64, error) {
	senderVoteCountKey := append(voteFromPrefix, address...)
	value, err := dao.kvstore.Get(blockAddressVoteCountMappingNS, senderVoteCountKey)
	if err != nil {
		return 0, nil
	}
	if len(value) == 0 {
		return 0, errors.New("count of votes as sender is broken")
	}
	return enc.MachineEndian.Uint64(value), nil
}

// TODO: To be deprecated
// getVotesByRecipientAddress returns votes by recipient address
func (dao *blockDAO) getVotesByRecipientAddress(address string) ([]hash.Hash32B, error) {
	recipientVoteCount, err := dao.getVoteCountByRecipientAddress(address)
	if err != nil {
		return nil, errors.Wrapf(err, "to get votecount for recipient %x", address)
	}

	res, err := dao.getVotesByAddress(address, recipientVoteCount, voteToPrefix)
	if err != nil {
		return nil, errors.Wrapf(err, "to get votes for recipient %x", address)
	}

	return res, nil
}

// TODO: To be deprecated
// getVotesByAddress returns votes by address
func (dao *blockDAO) getVotesByAddress(address string, count uint64, keyPrefix []byte) ([]hash.Hash32B, error) {
	var res []hash.Hash32B

	for i := uint64(0); i < count; i++ {
		// put new vote to recipient
		key := append(keyPrefix, address...)
		key = append(key, byteutil.Uint64ToBytes(i)...)
		value, err := dao.kvstore.Get(blockAddressVoteMappingNS, key)
		if err != nil {
			return res, errors.Wrapf(err, "failed to get vote for index %x", i)
		}
		if len(value) == 0 {
			return res, errors.Wrapf(db.ErrNotExist, "vote for index %x missing", i)
		}
		voteHash := hash.ZeroHash32B
		copy(voteHash[:], value)
		res = append(res, voteHash)
	}

	return res, nil
}

// TODO: To be deprecated
// getVoteCountByRecipientAddress returns vote count by recipient address
func (dao *blockDAO) getVoteCountByRecipientAddress(address string) (uint64, error) {
	recipientVoteCountKey := append(voteToPrefix, address...)
	value, err := dao.kvstore.Get(blockAddressVoteCountMappingNS, recipientVoteCountKey)
	if err != nil {
		return 0, nil
	}
	if len(value) == 0 {
		return 0, errors.New("count of votes as recipient is broken")
	}
	return enc.MachineEndian.Uint64(value), nil
}

// TODO: To be deprecated
// getExecutionsByExecutorAddress returns executions for executor
func (dao *blockDAO) getExecutionsByExecutorAddress(address string) ([]hash.Hash32B, error) {
	// get executions count for sender
	executorExecutionCount, err := dao.getExecutionCountByExecutorAddress(address)
	if err != nil {
		return nil, errors.Wrapf(err, "for executor %x", address)
	}

	res, getExecutionsErr := dao.getExecutionsByAddress(address, executorExecutionCount, executionFromPrefix)
	if getExecutionsErr != nil {
		return nil, getExecutionsErr
	}

	return res, nil
}

// TODO: To be deprecated
// getExecutionCountByExecutorAddress returns execution count by executor address
func (dao *blockDAO) getExecutionCountByExecutorAddress(address string) (uint64, error) {
	executorExecutionCountKey := append(executionFromPrefix, address...)
	value, err := dao.kvstore.Get(blockAddressExecutionCountMappingNS, executorExecutionCountKey)
	if err != nil {
		return 0, nil
	}
	if len(value) == 0 {
		return 0, errors.New("count of executions as contract is broken")
	}
	return enc.MachineEndian.Uint64(value), nil
}

// TODO: To be deprecated
// getExecutionsByContractAddress returns executions for contract
func (dao *blockDAO) getExecutionsByContractAddress(address string) ([]hash.Hash32B, error) {
	// get execution count for contract
	contractExecutionCount, getCountErr := dao.getExecutionCountByContractAddress(address)
	if getCountErr != nil {
		return nil, errors.Wrapf(getCountErr, "for contract %x", address)
	}

	res, getExecutionsErr := dao.getExecutionsByAddress(address, contractExecutionCount, executionToPrefix)
	if getExecutionsErr != nil {
		return nil, getExecutionsErr
	}

	return res, nil
}

// TODO: To be deprecated
// getExecutionsByAddress returns executions by address
func (dao *blockDAO) getExecutionsByAddress(address string, count uint64, keyPrefix []byte) ([]hash.Hash32B, error) {
	var res []hash.Hash32B

	for i := uint64(0); i < count; i++ {
		// put new execution to recipient
		key := append(keyPrefix, address...)
		key = append(key, byteutil.Uint64ToBytes(i)...)
		value, err := dao.kvstore.Get(blockAddressExecutionMappingNS, key)
		if err != nil {
			return res, errors.Wrapf(err, "failed to get execution for index %x", i)
		}
		if len(value) == 0 {
			return res, errors.Wrapf(db.ErrNotExist, "execution for index %x missing", i)
		}
		executionHash := hash.ZeroHash32B
		copy(executionHash[:], value)
		res = append(res, executionHash)
	}

	return res, nil
}

// TODO: To be deprecated
// getExecutionCountByContractAddress returns execution count by contract address
func (dao *blockDAO) getExecutionCountByContractAddress(address string) (uint64, error) {
	contractExecutionCountKey := append(executionToPrefix, address...)
	value, err := dao.kvstore.Get(blockAddressExecutionCountMappingNS, contractExecutionCountKey)
	if err != nil {
		return 0, nil
	}
	if len(value) == 0 {
		return 0, errors.New("count of executions as contract is broken")
	}
	return enc.MachineEndian.Uint64(value), nil
}

// getActionCountBySenderAddress returns action count by sender address
func (dao *blockDAO) getActionCountBySenderAddress(address string) (uint64, error) {
	senderActionCountKey := append(actionFromPrefix, address...)
	value, err := dao.kvstore.Get(blockAddressActionCountMappingNS, senderActionCountKey)
	if err != nil {
		return 0, nil
	}
	if len(value) == 0 {
		return 0, errors.New("count of actions by sender is broken")
	}
	return enc.MachineEndian.Uint64(value), nil
}

// getActionsBySenderAddress returns actions for sender
func (dao *blockDAO) getActionsBySenderAddress(address string) ([]hash.Hash32B, error) {
	// get action count for sender
	senderActionCount, err := dao.getActionCountBySenderAddress(address)
	if err != nil {
		return nil, errors.Wrapf(err, "for sender %x", address)
	}

	res, err := dao.getActionsByAddress(address, senderActionCount, actionFromPrefix)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// getActionsByRecipientAddress returns actions for recipient
func (dao *blockDAO) getActionsByRecipientAddress(address string) ([]hash.Hash32B, error) {
	// get action count for recipient
	recipientActionCount, getCountErr := dao.getActionCountByRecipientAddress(address)
	if getCountErr != nil {
		return nil, errors.Wrapf(getCountErr, "for recipient %x", address)
	}

	res, err := dao.getActionsByAddress(address, recipientActionCount, actionToPrefix)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// getActionsByAddress returns actions by address
func (dao *blockDAO) getActionsByAddress(address string, count uint64, keyPrefix []byte) ([]hash.Hash32B, error) {
	var res []hash.Hash32B

	for i := uint64(0); i < count; i++ {
		key := append(keyPrefix, address...)
		key = append(key, byteutil.Uint64ToBytes(i)...)
		value, err := dao.kvstore.Get(blockAddressActionMappingNS, key)
		if err != nil {
			return res, errors.Wrapf(err, "failed to get action for index %d", i)
		}
		if len(value) == 0 {
			return res, errors.Wrapf(db.ErrNotExist, "action for index %d missing", i)
		}
		actHash := hash.ZeroHash32B
		copy(actHash[:], value)
		res = append(res, actHash)
	}

	return res, nil
}

// getActionCountByRecipientAddress returns action count by recipient address
func (dao *blockDAO) getActionCountByRecipientAddress(address string) (uint64, error) {
	recipientActionCountKey := append(actionToPrefix, address...)
	value, err := dao.kvstore.Get(blockAddressActionCountMappingNS, recipientActionCountKey)
	if err != nil {
		return 0, nil
	}
	if len(value) == 0 {
		return 0, errors.New("count of actions by recipient is broken")
	}
	return enc.MachineEndian.Uint64(value), nil
}

// getBlockchainHeight returns the blockchain height
func (dao *blockDAO) getBlockchainHeight() (uint64, error) {
	value, err := dao.kvstore.Get(blockNS, topHeightKey)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get top height")
	}
	if len(value) == 0 {
		return 0, errors.Wrap(db.ErrNotExist, "blockchain height missing")
	}
	return enc.MachineEndian.Uint64(value), nil
}

// TODO: To be deprecated
// getTotalTransfers returns the total number of transfers
func (dao *blockDAO) getTotalTransfers() (uint64, error) {
	value, err := dao.kvstore.Get(blockNS, totalTransfersKey)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get total transfers")
	}
	if len(value) == 0 {
		return 0, errors.Wrap(db.ErrNotExist, "total transfers missing")
	}
	return enc.MachineEndian.Uint64(value), nil
}

// TODO: To be deprecated
// getTotalVotes returns the total number of votes
func (dao *blockDAO) getTotalVotes() (uint64, error) {
	value, err := dao.kvstore.Get(blockNS, totalVotesKey)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get total votes")
	}
	if len(value) == 0 {
		return 0, errors.Wrap(db.ErrNotExist, "total votes missing")
	}
	return enc.MachineEndian.Uint64(value), nil
}

// TODO: To be deprecated
// getTotalExecutions returns the total number of executions
func (dao *blockDAO) getTotalExecutions() (uint64, error) {
	value, err := dao.kvstore.Get(blockNS, totalExecutionsKey)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get total executions")
	}
	if len(value) == 0 {
		return 0, errors.Wrap(db.ErrNotExist, "total executions missing")
	}
	return enc.MachineEndian.Uint64(value), nil
}

// getTotalActions returns the total number of actions
func (dao *blockDAO) getTotalActions() (uint64, error) {
	value, err := dao.kvstore.Get(blockNS, totalActionsKey)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get total actions")
	}
	if len(value) == 0 {
		return 0, errors.Wrap(db.ErrNotExist, "total actions missing")
	}
	return enc.MachineEndian.Uint64(value), nil
}

// getReceiptByActionHash returns the receipt by execution hash
func (dao *blockDAO) getReceiptByActionHash(h hash.Hash32B) (*action.Receipt, error) {
	heightBytes, err := dao.kvstore.Get(blockActionReceiptMappingNS, h[:])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get receipt index for action %x", h)
	}
	receiptsBytes, err := dao.kvstore.Get(receiptsNS, heightBytes)
	if err != nil {
		height := enc.MachineEndian.Uint64(heightBytes)
		return nil, errors.Wrapf(err, "failed to get receipts of block %d", height)
	}
	receipts := iproto.Receipts{}
	if err := proto.Unmarshal(receiptsBytes, &receipts); err != nil {
		return nil, err
	}
	for _, receipt := range receipts.Receipts {
		r := action.Receipt{}
		r.ConvertFromReceiptPb(receipt)
		if r.Hash == h {
			return &r, nil
		}
	}
	return nil, errors.Errorf("receipt of action %x isn't found", h)
}

// putBlock puts a block
func (dao *blockDAO) putBlock(blk *block.Block) error {
	batch := db.NewBatch()

	height := byteutil.Uint64ToBytes(blk.Height())

	serialized, err := blk.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize block")
	}
	hash := blk.HashBlock()
	batch.Put(blockNS, hash[:], serialized, "failed to put block")

	hashKey := append(hashPrefix, hash[:]...)
	batch.Put(blockHashHeightMappingNS, hashKey, height, "failed to put hash -> height mapping")

	heightKey := append(heightPrefix, height...)
	batch.Put(blockHashHeightMappingNS, heightKey, hash[:], "failed to put height -> hash mapping")

	value, err := dao.kvstore.Get(blockNS, topHeightKey)
	if err != nil {
		return errors.Wrap(err, "failed to get top height")
	}
	topHeight := enc.MachineEndian.Uint64(value)
	if blk.Height() > topHeight {
		batch.Put(blockNS, topHeightKey, height, "failed to put top height")
	}

	if !dao.writeIndex {
		return dao.kvstore.Commit(batch)
	}

	// TODO: To be deprecated
	// only build Tsf/Vote/Execution index if enable explorer
	transfers, votes, executions := action.ClassifyActions(blk.Actions)
	value, err = dao.kvstore.Get(blockNS, totalTransfersKey)
	if err != nil {
		return errors.Wrap(err, "failed to get total transfers")
	}
	totalTransfers := enc.MachineEndian.Uint64(value)
	totalTransfers += uint64(len(transfers))
	totalTransfersBytes := byteutil.Uint64ToBytes(totalTransfers)
	batch.Put(blockNS, totalTransfersKey, totalTransfersBytes, "failed to put total transfers")

	value, err = dao.kvstore.Get(blockNS, totalVotesKey)
	if err != nil {
		return errors.Wrap(err, "failed to get total votes")
	}
	totalVotes := enc.MachineEndian.Uint64(value)
	totalVotes += uint64(len(votes))
	totalVotesBytes := byteutil.Uint64ToBytes(totalVotes)
	batch.Put(blockNS, totalVotesKey, totalVotesBytes, "failed to put total votes")

	value, err = dao.kvstore.Get(blockNS, totalExecutionsKey)
	if err != nil {
		return errors.Wrap(err, "failed to get total executions")
	}
	totalExecutions := enc.MachineEndian.Uint64(value)
	totalExecutions += uint64(len(executions))
	totalExecutionsBytes := byteutil.Uint64ToBytes(totalExecutions)
	batch.Put(blockNS, totalExecutionsKey, totalExecutionsBytes, "failed to put total executions")

	value, err = dao.kvstore.Get(blockNS, totalActionsKey)
	if err != nil {
		return errors.Wrap(err, "failed to get total actions")
	}
	totalActions := enc.MachineEndian.Uint64(value)
	totalActions += uint64(len(blk.Actions) - len(transfers) - len(votes) - len(executions))
	totalActionsBytes := byteutil.Uint64ToBytes(totalActions)
	batch.Put(blockNS, totalActionsKey, totalActionsBytes, "failed to put total actions")

	for _, elp := range blk.Actions {
		var (
			prefix []byte
			ns     string
		)
		act := elp.Action()
		// TODO: To be deprecated
		switch act.(type) {
		case *action.Transfer:
			prefix = transferPrefix
			ns = blockTransferBlockMappingNS
		case *action.Vote:
			prefix = votePrefix
			ns = blockVoteBlockMappingNS
		case *action.Execution:
			prefix = executionPrefix
			ns = blockExecutionBlockMappingNS
		}
		actHash := elp.Hash()
		if prefix != nil {
			hashKey := append(prefix, actHash[:]...)
			batch.Put(ns, hashKey, hash[:], "failed to put action hash %x", actHash)
		}
		actionHashKey := append(actionPrefix, actHash[:]...)
		batch.Put(blockActionBlockMappingNS, actionHashKey, hash[:], "failed to put action hash %x", actHash)
	}

	// TODO: To be deprecated
	if err = putTransfers(dao, blk, batch); err != nil {
		return err
	}

	// TODO: To be deprecated
	if err = putVotes(dao, blk, batch); err != nil {
		return err
	}

	// TODO: To be deprecated
	if err = putExecutions(dao, blk, batch); err != nil {
		return err
	}

	if err = putActions(dao, blk, batch); err != nil {
		return err
	}
	return dao.kvstore.Commit(batch)
}

// TODO: To be deprecated
// putTransfers stores transfer information into db
func putTransfers(dao *blockDAO, blk *block.Block, batch db.KVStoreBatch) error {
	senderDelta := map[string]uint64{}
	recipientDelta := map[string]uint64{}

	transfers, _, _ := action.ClassifyActions(blk.Actions)
	for _, transfer := range transfers {
		transferHash := transfer.Hash()

		// get transfers count for sender
		senderTransferCount, err := dao.getTransferCountBySenderAddress(transfer.Sender())
		if err != nil {
			return errors.Wrapf(err, "for sender %x", transfer.Sender())
		}
		if delta, ok := senderDelta[transfer.Sender()]; ok {
			senderTransferCount += delta
			senderDelta[transfer.Sender()] = senderDelta[transfer.Sender()] + 1
		} else {
			senderDelta[transfer.Sender()] = 1
		}

		// put new transfer to sender
		senderKey := append(transferFromPrefix, transfer.Sender()...)
		senderKey = append(senderKey, byteutil.Uint64ToBytes(senderTransferCount)...)
		batch.Put(blockAddressTransferMappingNS, senderKey, transferHash[:],
			"failed to put transfer hash %x for sender %x", transfer.Hash(), transfer.Sender())

		// update sender transfers count
		senderTransferCountKey := append(transferFromPrefix, transfer.Sender()...)
		batch.Put(blockAddressTransferCountMappingNS, senderTransferCountKey,
			byteutil.Uint64ToBytes(senderTransferCount+1), "failed to bump transfer count %x for sender %x",
			transfer.Hash(), transfer.Sender())

		// get transfers count for recipient
		recipientTransferCount, err := dao.getTransferCountByRecipientAddress(transfer.Recipient())
		if err != nil {
			return errors.Wrapf(err, "for recipient %x", transfer.Recipient())
		}
		if delta, ok := recipientDelta[transfer.Recipient()]; ok {
			recipientTransferCount += delta
			recipientDelta[transfer.Recipient()] = recipientDelta[transfer.Recipient()] + 1
		} else {
			recipientDelta[transfer.Recipient()] = 1
		}

		// put new transfer to recipient
		recipientKey := append(transferToPrefix, transfer.Recipient()...)
		recipientKey = append(recipientKey, byteutil.Uint64ToBytes(recipientTransferCount)...)

		batch.Put(blockAddressTransferMappingNS, recipientKey, transferHash[:],
			"failed to put transfer hash %x for recipient %x", transfer.Hash(), transfer.Recipient())

		// update recipient transfers count
		recipientTransferCountKey := append(transferToPrefix, transfer.Recipient()...)
		batch.Put(blockAddressTransferCountMappingNS, recipientTransferCountKey,
			byteutil.Uint64ToBytes(recipientTransferCount+1), "failed to bump transfer count %x for recipient %x",
			transfer.Hash(), transfer.Recipient())
	}

	return nil
}

// TODO: To be deprecated
// putVotes stores vote information into db
func putVotes(dao *blockDAO, blk *block.Block, batch db.KVStoreBatch) error {
	senderDelta := map[string]uint64{}
	recipientDelta := map[string]uint64{}

	for _, selp := range blk.Actions {
		vote, ok := selp.Action().(*action.Vote)
		if !ok {
			continue
		}
		voteHash := selp.Hash()
		Sender := vote.Voter()
		Recipient := vote.Votee()

		// get votes count for sender
		senderVoteCount, err := dao.getVoteCountBySenderAddress(Sender)
		if err != nil {
			return errors.Wrapf(err, "for sender %x", Sender)
		}
		if delta, ok := senderDelta[Sender]; ok {
			senderVoteCount += delta
			senderDelta[Sender] = senderDelta[Sender] + 1
		} else {
			senderDelta[Sender] = 1
		}

		// put new vote to sender
		senderKey := append(voteFromPrefix, Sender...)
		senderKey = append(senderKey, byteutil.Uint64ToBytes(senderVoteCount)...)
		batch.Put(blockAddressVoteMappingNS, senderKey, voteHash[:],
			"failed to put vote hash %x for sender %x", voteHash, Sender)

		// update sender votes count
		senderVoteCountKey := append(voteFromPrefix, Sender...)
		batch.Put(blockAddressVoteCountMappingNS, senderVoteCountKey,
			byteutil.Uint64ToBytes(senderVoteCount+1), "failed to bump vote count %x for sender %x",
			voteHash, Sender)

		// get votes count for recipient
		recipientVoteCount, err := dao.getVoteCountByRecipientAddress(Recipient)
		if err != nil {
			return errors.Wrapf(err, "for recipient %x", Recipient)
		}
		if delta, ok := recipientDelta[Recipient]; ok {
			recipientVoteCount += delta
			recipientDelta[Recipient] = recipientDelta[Recipient] + 1
		} else {
			recipientDelta[Recipient] = 1
		}

		// put new vote to recipient
		recipientKey := append(voteToPrefix, Recipient...)
		recipientKey = append(recipientKey, byteutil.Uint64ToBytes(recipientVoteCount)...)
		batch.Put(blockAddressVoteMappingNS, recipientKey, voteHash[:],
			"failed to put vote hash %x for recipient %x", voteHash, Recipient)

		// update recipient votes count
		recipientVoteCountKey := append(voteToPrefix, Recipient...)
		batch.Put(blockAddressVoteCountMappingNS, recipientVoteCountKey,
			byteutil.Uint64ToBytes(recipientVoteCount+1),
			"failed to bump vote count %x for recipient %x", voteHash, Recipient)
	}

	return nil
}

// TODO: To be deprecated
// putExecutions stores execution information into db
func putExecutions(dao *blockDAO, blk *block.Block, batch db.KVStoreBatch) error {
	executorDelta := map[string]uint64{}
	contractDelta := map[string]uint64{}

	_, _, executions := action.ClassifyActions(blk.Actions)
	for _, execution := range executions {
		executionHash := execution.Hash()

		// get execution count for executor
		executorExecutionCount, err := dao.getExecutionCountByExecutorAddress(execution.Executor())
		if err != nil {
			return errors.Wrapf(err, "for executor %x", execution.Executor())
		}
		if delta, ok := executorDelta[execution.Executor()]; ok {
			executorExecutionCount += delta
			executorDelta[execution.Executor()] = executorDelta[execution.Executor()] + 1
		} else {
			executorDelta[execution.Executor()] = 1
		}

		// put new execution to executor
		executorKey := append(executionFromPrefix, execution.Executor()...)
		executorKey = append(executorKey, byteutil.Uint64ToBytes(executorExecutionCount)...)
		batch.Put(blockAddressExecutionMappingNS, executorKey, executionHash[:],
			"failed to put execution hash %x for executor %x", execution.Hash(), execution.Executor())

		// update executor executions count
		executorExecutionCountKey := append(executionFromPrefix, execution.Executor()...)
		batch.Put(blockAddressExecutionCountMappingNS, executorExecutionCountKey,
			byteutil.Uint64ToBytes(executorExecutionCount+1),
			"failed to bump execution count %x for executor %x", execution.Hash(), execution.Executor())

		// get execution count for contract
		contractExecutionCount, err := dao.getExecutionCountByContractAddress(execution.Contract())
		if err != nil {
			return errors.Wrapf(err, "for contract %x", execution.Contract())
		}
		if delta, ok := contractDelta[execution.Contract()]; ok {
			contractExecutionCount += delta
			contractDelta[execution.Contract()] = contractDelta[execution.Contract()] + 1
		} else {
			contractDelta[execution.Contract()] = 1
		}

		// put new execution to contract
		contractKey := append(executionToPrefix, execution.Contract()...)
		contractKey = append(contractKey, byteutil.Uint64ToBytes(contractExecutionCount)...)
		batch.Put(blockAddressExecutionMappingNS, contractKey, executionHash[:],
			"failed to put execution hash %x for contract %x", execution.Hash(), execution.Contract())

		// update contract executions count
		contractExecutionCountKey := append(executionToPrefix, execution.Contract()...)
		batch.Put(blockAddressExecutionCountMappingNS, contractExecutionCountKey,
			byteutil.Uint64ToBytes(contractExecutionCount+1), "failed to bump execution count %x for contract %x",
			execution.Hash(), execution.Contract())
	}
	return nil
}

func putActions(dao *blockDAO, blk *block.Block, batch db.KVStoreBatch) error {
	senderDelta := make(map[string]uint64)
	recipientDelta := make(map[string]uint64)

	for _, selp := range blk.Actions {
		actHash := selp.Hash()

		// get action count for sender
		senderActionCount, err := dao.getActionCountBySenderAddress(selp.SrcAddr())
		if err != nil {
			return errors.Wrapf(err, "for sender %s", selp.SrcAddr())
		}
		if delta, ok := senderDelta[selp.SrcAddr()]; ok {
			senderActionCount += delta
			senderDelta[selp.SrcAddr()]++
		} else {
			senderDelta[selp.SrcAddr()] = 1
		}

		// put new action to sender
		senderKey := append(actionFromPrefix, selp.SrcAddr()...)
		senderKey = append(senderKey, byteutil.Uint64ToBytes(senderActionCount)...)
		batch.Put(blockAddressActionMappingNS, senderKey, actHash[:],
			"failed to put action hash %x for sender %s", actHash, selp.SrcAddr())

		// update sender action count
		senderActionCountKey := append(actionFromPrefix, selp.SrcAddr()...)
		batch.Put(blockAddressActionCountMappingNS, senderActionCountKey,
			byteutil.Uint64ToBytes(senderActionCount+1),
			"failed to bump action count %x for sender %s", actHash, selp.SrcAddr())

		// get action count for recipient
		recipientActionCount, err := dao.getActionCountByRecipientAddress(selp.DstAddr())
		if err != nil {
			return errors.Wrapf(err, "for recipient %s", selp.DstAddr())
		}
		if delta, ok := recipientDelta[selp.DstAddr()]; ok {
			recipientActionCount += delta
			recipientDelta[selp.DstAddr()]++
		} else {
			recipientDelta[selp.DstAddr()] = 1
		}

		// put new action to recipient
		recipientKey := append(actionToPrefix, selp.DstAddr()...)
		recipientKey = append(recipientKey, byteutil.Uint64ToBytes(recipientActionCount)...)
		batch.Put(blockAddressActionMappingNS, recipientKey, actHash[:],
			"failed to put action hash %x for recipient %s", actHash, selp.DstAddr())

		// update recipient action count
		recipientActionCountKey := append(actionToPrefix, selp.DstAddr()...)
		batch.Put(blockAddressActionCountMappingNS, recipientActionCountKey,
			byteutil.Uint64ToBytes(recipientActionCount+1), "failed to bump action count %x for recipient %s",
			actHash, selp.DstAddr())
	}
	return nil
}

// putReceipts store receipt into db
func (dao *blockDAO) putReceipts(blkHeight uint64, blkReceipts []*action.Receipt) error {
	if blkReceipts == nil {
		return nil
	}
	receipts := iproto.Receipts{}
	batch := db.NewBatch()
	var heightBytes [8]byte
	enc.MachineEndian.PutUint64(heightBytes[:], blkHeight)
	for _, r := range blkReceipts {
		receipts.Receipts = append(receipts.Receipts, r.ConvertToReceiptPb())
		batch.Put(
			blockActionReceiptMappingNS,
			r.Hash[:],
			heightBytes[:],
			"Failed to put receipt index for action %x",
			r.Hash[:],
		)
	}
	receiptsBytes, err := proto.Marshal(&receipts)
	if err != nil {
		return err
	}
	batch.Put(receiptsNS, heightBytes[:], receiptsBytes, "Failed to put receipts of block %d", blkHeight)
	return dao.kvstore.Commit(batch)
}

// deleteBlock deletes the tip block
func (dao *blockDAO) deleteTipBlock() error {
	batch := db.NewBatch()

	// First obtain tip height from db
	heightValue, err := dao.kvstore.Get(blockNS, topHeightKey)
	if err != nil {
		return errors.Wrap(err, "failed to get tip height")
	}

	// Obtain tip block hash
	hash, err := dao.getBlockHash(enc.MachineEndian.Uint64(heightValue))
	if err != nil {
		return errors.Wrap(err, "failed to get tip block hash")
	}

	// Obtain block
	blk, err := dao.getBlock(hash)
	if err != nil {
		return errors.Wrap(err, "failed to get tip block")
	}

	// Delete hash -> block mapping
	batch.Delete(blockNS, hash[:], "failed to delete block")

	// Delete hash -> height mapping
	hashKey := append(hashPrefix, hash[:]...)
	batch.Delete(blockHashHeightMappingNS, hashKey, "failed to delete hash -> height mapping")

	// Delete height -> hash mapping
	heightKey := append(heightPrefix, heightValue...)
	batch.Delete(blockHashHeightMappingNS, heightKey, "failed to delete height -> hash mapping")

	// Update tip height
	topHeight := enc.MachineEndian.Uint64(heightValue) - 1
	topHeightValue := byteutil.Uint64ToBytes(topHeight)
	batch.Put(blockNS, topHeightKey, topHeightValue, "failed to put top height")

	if !dao.writeIndex {
		return dao.kvstore.Commit(batch)
	}

	// TODO: To be deprecated
	// Only delete Tsf/Vote/Execution index if enable explorer
	transfers, votes, executions := action.ClassifyActions(blk.Actions)
	// Update total transfer count
	value, err := dao.kvstore.Get(blockNS, totalTransfersKey)
	if err != nil {
		return errors.Wrap(err, "failed to get total transfers")
	}
	totalTransfers := enc.MachineEndian.Uint64(value)
	totalTransfers -= uint64(len(transfers))
	totalTransfersBytes := byteutil.Uint64ToBytes(totalTransfers)
	batch.Put(blockNS, totalTransfersKey, totalTransfersBytes, "failed to put total transfers")

	// Update total vote count
	value, err = dao.kvstore.Get(blockNS, totalVotesKey)
	if err != nil {
		return errors.Wrap(err, "failed to get total votes")
	}
	totalVotes := enc.MachineEndian.Uint64(value)
	totalVotes -= uint64(len(votes))
	totalVotesBytes := byteutil.Uint64ToBytes(totalVotes)
	batch.Put(blockNS, totalVotesKey, totalVotesBytes, "failed to put total votes")

	// Update total execution count
	value, err = dao.kvstore.Get(blockNS, totalExecutionsKey)
	if err != nil {
		return errors.Wrap(err, "failed to get total executions")
	}
	totalExecutions := enc.MachineEndian.Uint64(value)
	totalExecutions -= uint64(len(executions))
	totalExecutionsBytes := byteutil.Uint64ToBytes(totalExecutions)
	batch.Put(blockNS, totalExecutionsKey, totalExecutionsBytes, "failed to put total executions")

	// update total action count
	value, err = dao.kvstore.Get(blockNS, totalActionsKey)
	if err != nil {
		return errors.Wrap(err, "failed to get total actions")
	}
	totalActions := enc.MachineEndian.Uint64(value)
	totalActions -= uint64(len(blk.Actions))
	totalActionsBytes := byteutil.Uint64ToBytes(totalActions)
	batch.Put(blockNS, totalActionsKey, totalActionsBytes, "failed to put total actions")

	// TODO: To be deprecated
	// Delete transfer hash -> block hash mapping
	for _, transfer := range transfers {
		transferHash := transfer.Hash()
		hashKey := append(transferPrefix, transferHash[:]...)
		batch.Delete(blockTransferBlockMappingNS, hashKey, "failed to delete transfer hash %x", transferHash)
	}

	// TODO: To be deprecated
	// Delete vote hash -> block hash mapping
	for _, vote := range votes {
		voteHash := vote.Hash()
		hashKey := append(votePrefix, voteHash[:]...)
		batch.Delete(blockVoteBlockMappingNS, hashKey, "failed to delete vote hash %x", voteHash)
	}

	// TODO: To be deprecated
	// Delete execution hash -> block hash mapping
	for _, execution := range executions {
		executionHash := execution.Hash()
		hashKey := append(executionPrefix, executionHash[:]...)
		batch.Delete(blockExecutionBlockMappingNS, hashKey, "failed to delete execution hash %x", executionHash)
	}

	// Delete action hash -> block hash mapping
	for _, selp := range blk.Actions {
		actHash := selp.Hash()
		hashKey := append(actionPrefix, actHash[:]...)
		batch.Delete(blockActionBlockMappingNS, hashKey, "failed to delete actions f")
	}

	// TODO: To be deprecated
	if err = deleteTransfers(dao, blk, batch); err != nil {
		return err
	}

	// TODO: To be deprecated
	if err = deleteVotes(dao, blk, batch); err != nil {
		return err
	}

	// TODO: To be deprecated
	if err = deleteExecutions(dao, blk, batch); err != nil {
		return err
	}

	if err = deleteActions(dao, blk, batch); err != nil {
		return err
	}

	if err = deleteReceipts(blk, batch); err != nil {
		return err
	}

	return dao.kvstore.Commit(batch)
}

// TODO: To be deprecated
// deleteTransfers deletes transfer information from db
func deleteTransfers(dao *blockDAO, blk *block.Block, batch db.KVStoreBatch) error {
	transfers, _, _ := action.ClassifyActions(blk.Actions)
	// First get the total count of transfers by sender and recipient respectively in the block
	senderCount := make(map[string]uint64)
	recipientCount := make(map[string]uint64)
	for _, transfer := range transfers {
		senderCount[transfer.Sender()]++
		recipientCount[transfer.Recipient()]++
	}
	// Roll back the status of address -> transferCount mapping to the previous block
	for sender, count := range senderCount {
		senderTransferCount, err := dao.getTransferCountBySenderAddress(sender)
		if err != nil {
			return errors.Wrapf(err, "for sender %x", sender)
		}
		senderTransferCountKey := append(transferFromPrefix, sender...)
		senderCount[sender] = senderTransferCount - count
		batch.Put(blockAddressTransferCountMappingNS, senderTransferCountKey,
			byteutil.Uint64ToBytes(senderCount[sender]), "failed to update transfer count for sender %x", sender)
	}
	for recipient, count := range recipientCount {
		recipientTransferCount, err := dao.getTransferCountByRecipientAddress(recipient)
		if err != nil {
			return errors.Wrapf(err, "for recipient %x", recipient)
		}
		recipientTransferCountKey := append(transferToPrefix, recipient...)
		recipientCount[recipient] = recipientTransferCount - count
		batch.Put(blockAddressTransferCountMappingNS, recipientTransferCountKey,
			byteutil.Uint64ToBytes(recipientCount[recipient]),
			"failed to update transfer count for recipient %x", recipient)
	}

	senderDelta := map[string]uint64{}
	recipientDelta := map[string]uint64{}

	for _, transfer := range transfers {
		transferHash := transfer.Hash()

		if delta, ok := senderDelta[transfer.Sender()]; ok {
			senderCount[transfer.Sender()] += delta
			senderDelta[transfer.Sender()] = senderDelta[transfer.Sender()] + 1
		} else {
			senderDelta[transfer.Sender()] = 1
		}

		// Delete new transfer from sender
		senderKey := append(transferFromPrefix, transfer.Sender()...)
		senderKey = append(senderKey, byteutil.Uint64ToBytes(senderCount[transfer.Sender()])...)
		batch.Delete(blockAddressTransferMappingNS, senderKey, "failed to delete transfer hash %x for sender %x",
			transfer.Hash(), transfer.Sender())

		if delta, ok := recipientDelta[transfer.Recipient()]; ok {
			recipientCount[transfer.Recipient()] += delta
			recipientDelta[transfer.Recipient()] = recipientDelta[transfer.Recipient()] + 1
		} else {
			recipientDelta[transfer.Recipient()] = 1
		}

		// Delete new transfer to recipient
		recipientKey := append(transferToPrefix, transfer.Recipient()...)
		recipientKey = append(recipientKey, byteutil.Uint64ToBytes(recipientCount[transfer.Recipient()])...)
		batch.Delete(blockAddressTransferMappingNS, recipientKey, "failed to delete transfer hash %x for recipient %x",
			transferHash, transfer.Recipient())
	}

	return nil
}

// TODO: To be deprecated
// deleteVotes deletes vote information from db
func deleteVotes(dao *blockDAO, blk *block.Block, batch db.KVStoreBatch) error {
	_, votes, _ := action.ClassifyActions(blk.Actions)
	// First get the total count of votes by sender and recipient respectively in the block
	senderCount := make(map[string]uint64)
	recipientCount := make(map[string]uint64)
	for _, vote := range votes {
		sender := vote.Voter()
		recipient := vote.Votee()
		senderCount[sender]++
		recipientCount[recipient]++
	}
	// Roll back the status of address -> voteCount mapping to the previous block
	for sender, count := range senderCount {
		senderVoteCount, err := dao.getVoteCountBySenderAddress(sender)
		if err != nil {
			return errors.Wrapf(err, "for sender %x", sender)
		}
		senderVoteCountKey := append(voteFromPrefix, sender...)
		senderCount[sender] = senderVoteCount - count
		batch.Put(blockAddressVoteCountMappingNS, senderVoteCountKey, byteutil.Uint64ToBytes(senderCount[sender]),
			"failed to update vote count for sender %x", sender)
	}
	for recipient, count := range recipientCount {
		recipientVoteCount, err := dao.getVoteCountByRecipientAddress(recipient)
		if err != nil {
			return errors.Wrapf(err, "for recipient %x", recipient)
		}
		recipientVoteCountKey := append(voteToPrefix, recipient...)
		recipientCount[recipient] = recipientVoteCount - count
		batch.Put(blockAddressVoteCountMappingNS, recipientVoteCountKey,
			byteutil.Uint64ToBytes(recipientCount[recipient]), "failed to update vote count for recipient %x", recipient)
	}

	senderDelta := map[string]uint64{}
	recipientDelta := map[string]uint64{}

	for _, vote := range votes {
		voteHash := vote.Hash()
		Sender := vote.Voter()
		Recipient := vote.Votee()

		if delta, ok := senderDelta[Sender]; ok {
			senderCount[Sender] += delta
			senderDelta[Sender] = senderDelta[Sender] + 1
		} else {
			senderDelta[Sender] = 1
		}

		// Delete new vote from sender
		senderKey := append(voteFromPrefix, Sender...)
		senderKey = append(senderKey, byteutil.Uint64ToBytes(senderCount[Sender])...)
		batch.Delete(blockAddressVoteMappingNS, senderKey, "failed to delete vote hash %x for sender %x",
			voteHash, Sender)

		if delta, ok := recipientDelta[Recipient]; ok {
			recipientCount[Recipient] += delta
			recipientDelta[Recipient] = recipientDelta[Recipient] + 1
		} else {
			recipientDelta[Recipient] = 1
		}

		// Delete new vote to recipient
		recipientKey := append(voteToPrefix, Recipient...)
		recipientKey = append(recipientKey, byteutil.Uint64ToBytes(recipientCount[Recipient])...)
		batch.Delete(blockAddressVoteMappingNS, recipientKey, "failed to delete vote hash %x for recipient %x",
			voteHash, Recipient)
	}

	return nil
}

// TODO: To be deprecated
// deleteExecutions deletes execution information from db
func deleteExecutions(dao *blockDAO, blk *block.Block, batch db.KVStoreBatch) error {
	_, _, executions := action.ClassifyActions(blk.Actions)
	// First get the total count of executions by executor and contract respectively in the block
	executorCount := make(map[string]uint64)
	contractCount := make(map[string]uint64)
	for _, execution := range executions {
		executorCount[execution.Executor()]++
		contractCount[execution.Contract()]++
	}
	// Roll back the status of address -> executionCount mapping to the previous block
	for executor, count := range executorCount {
		executorExecutionCount, err := dao.getExecutionCountByExecutorAddress(executor)
		if err != nil {
			return errors.Wrapf(err, "for executor %x", executor)
		}
		executorExecutionCountKey := append(executionFromPrefix, executor...)
		executorCount[executor] = executorExecutionCount - count
		batch.Put(blockAddressExecutionCountMappingNS, executorExecutionCountKey,
			byteutil.Uint64ToBytes(executorCount[executor]),
			"failed to update execution count for executor %x", executor)
	}
	for contract, count := range contractCount {
		contractExecutionCount, err := dao.getExecutionCountByContractAddress(contract)
		if err != nil {
			return errors.Wrapf(err, "for contract %x", contract)
		}
		contractExecutionCountKey := append(executionToPrefix, contract...)
		contractCount[contract] = contractExecutionCount - count
		batch.Put(blockAddressExecutionCountMappingNS, contractExecutionCountKey,
			byteutil.Uint64ToBytes(contractCount[contract]), "failed to update execution count for contract %x", contract)
	}

	executorDelta := map[string]uint64{}
	contractDelta := map[string]uint64{}

	for _, execution := range executions {
		executionHash := execution.Hash()

		if delta, ok := executorDelta[execution.Executor()]; ok {
			executorCount[execution.Executor()] += delta
			executorDelta[execution.Executor()] = executorDelta[execution.Executor()] + 1
		} else {
			executorDelta[execution.Executor()] = 1
		}

		// Delete new execution from executor
		executorKey := append(executionFromPrefix, execution.Executor()...)
		executorKey = append(executorKey, byteutil.Uint64ToBytes(executorCount[execution.Executor()])...)
		batch.Delete(blockAddressExecutionMappingNS, executorKey, "failed to delete execution hash %x for executor %x",
			execution.Hash(), execution.Executor())

		if delta, ok := contractDelta[execution.Contract()]; ok {
			contractCount[execution.Contract()] += delta
			contractDelta[execution.Contract()] = contractDelta[execution.Contract()] + 1
		} else {
			contractDelta[execution.Contract()] = 1
		}

		// Delete new execution to contract
		contractKey := append(executionToPrefix, execution.Contract()...)
		contractKey = append(contractKey, byteutil.Uint64ToBytes(contractCount[execution.Contract()])...)
		batch.Delete(blockAddressExecutionMappingNS, contractKey, "failed to delete execution hash %x for contract %x",
			executionHash, execution.Contract())
	}

	return nil
}

// deleteReceipts deletes receipt information from db
func deleteReceipts(blk *block.Block, batch db.KVStoreBatch) error {
	for _, r := range blk.Receipts {
		batch.Delete(blockActionReceiptMappingNS, r.Hash[:], "failed to delete receipt for action %x", r.Hash[:])
	}
	return nil
}

// deleteActions deletes action information from db
func deleteActions(dao *blockDAO, blk *block.Block, batch db.KVStoreBatch) error {
	// Firt get the total count of actions by sender and recipient respectively in the block
	senderCount := make(map[string]uint64)
	recipientCount := make(map[string]uint64)
	for _, selp := range blk.Actions {
		senderCount[selp.SrcAddr()]++
		recipientCount[selp.DstAddr()]++
	}
	// Roll back the status of address -> actionCount mapping to the preivous block
	for sender, count := range senderCount {
		senderActionCount, err := dao.getActionCountBySenderAddress(sender)
		if err != nil {
			return errors.Wrapf(err, "for sender %x", sender)
		}
		senderActionCountKey := append(actionFromPrefix, sender...)
		senderCount[sender] = senderActionCount - count
		batch.Put(blockAddressActionCountMappingNS, senderActionCountKey, byteutil.Uint64ToBytes(senderCount[sender]),
			"failed to update action count for sender %x", sender)
	}
	for recipient, count := range recipientCount {
		recipientActionCount, err := dao.getActionCountByRecipientAddress(recipient)
		if err != nil {
			return errors.Wrapf(err, "for recipient %x", recipient)
		}
		recipientActionCountKey := append(actionToPrefix, recipient...)
		recipientCount[recipient] = recipientActionCount - count
		batch.Put(blockAddressActionCountMappingNS, recipientActionCountKey,
			byteutil.Uint64ToBytes(recipientCount[recipient]), "failed to update action count for recipient %x",
			recipient)

	}

	senderDelta := map[string]uint64{}
	recipientDelta := map[string]uint64{}

	for _, selp := range blk.Actions {
		actHash := selp.Hash()

		if delta, ok := senderDelta[selp.SrcAddr()]; ok {
			senderCount[selp.SrcAddr()] += delta
			senderDelta[selp.SrcAddr()] = senderDelta[selp.SrcAddr()] + 1
		} else {
			senderDelta[selp.SrcAddr()] = 1
		}

		// Delete new action from sender
		senderKey := append(actionFromPrefix, selp.SrcAddr()...)
		senderKey = append(senderKey, byteutil.Uint64ToBytes(senderCount[selp.SrcAddr()])...)
		batch.Delete(blockAddressActionMappingNS, senderKey, "failed to delete action hash %x for sender %x",
			actHash, selp.SrcAddr())

		if delta, ok := recipientDelta[selp.DstAddr()]; ok {
			recipientCount[selp.DstAddr()] += delta
			recipientDelta[selp.DstAddr()] = recipientDelta[selp.DstAddr()] + 1
		} else {
			recipientDelta[selp.DstAddr()] = 1
		}

		// Delete new action to recipient
		recipientKey := append(actionToPrefix, selp.DstAddr()...)
		recipientKey = append(recipientKey, byteutil.Uint64ToBytes(recipientCount[selp.DstAddr()])...)
		batch.Delete(blockAddressActionMappingNS, recipientKey, "failed to delete action hash %x for recipient %x",
			actHash, selp.DstAddr())
	}

	return nil
}
