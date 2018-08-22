// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const (
	blockNS                             = "blocks"
	blockHashHeightMappingNS            = "hash<->height"
	blockTransferBlockMappingNS         = "transfer<->block"
	blockVoteBlockMappingNS             = "vote<->block"
	blockExecutionBlockMappingNS        = "execution<->block"
	blockExecutionReceiptMappingNS      = "ex<->receipt"
	blockAddressTransferMappingNS       = "address<->transfer"
	blockAddressTransferCountMappingNS  = "address<->transfercount"
	blockAddressVoteMappingNS           = "address<->vote"
	blockAddressVoteCountMappingNS      = "address<->votecount"
	blockAddressExecutionMappingNS      = "address<->execution"
	blockAddressExecutionCountMappingNS = "address<->executioncount"
)

var (
	hashPrefix      = []byte("hash.")
	transferPrefix  = []byte("transfer.")
	votePrefix      = []byte("vote.")
	executionPrefix = []byte("execution.")
	heightPrefix    = []byte("height.")
	// mutate this field is not thread safe, pls only mutate it in putBlock!
	topHeightKey = []byte("top-height")
	// mutate this field is not thread safe, pls only mutate it in putBlock!
	totalTransfersKey   = []byte("total-transfers")
	totalVotesKey       = []byte("total-votes")
	totalExecutionsKey  = []byte("total-executions")
	transferFromPrefix  = []byte("transfer-from.")
	transferToPrefix    = []byte("transfer-to.")
	voteFromPrefix      = []byte("vote-from.")
	voteToPrefix        = []byte("vote-to.")
	executionFromPrefix = []byte("execution-from")
	executionToPrefix   = []byte("execution-to")
)

var _ lifecycle.StartStopper = (*blockDAO)(nil)

type blockDAO struct {
	kvstore   db.KVStore
	lifecycle lifecycle.Lifecycle
}

// newBlockDAO instantiates a block DAO
func newBlockDAO(kvstore db.KVStore) *blockDAO {
	blockDAO := &blockDAO{kvstore: kvstore}
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
	if err := dao.kvstore.PutIfNotExists(blockNS, topHeightKey, make([]byte, 8)); err != nil {
		// ok on none-fresh db
		if err == db.ErrAlreadyExist {
			return nil
		}

		return errors.Wrap(err, "failed to write initial value for top height")
	}

	// set init total transfer to be 0
	if err = dao.kvstore.PutIfNotExists(blockNS, totalTransfersKey, make([]byte, 8)); err != nil {
		return errors.Wrap(err, "failed to write initial value for total transfers")
	}

	// set init total vote to be 0
	if err = dao.kvstore.PutIfNotExists(blockNS, totalVotesKey, make([]byte, 8)); err != nil {
		return errors.Wrap(err, "failed to write initial value for total votes")
	}

	// set init total executions to be 0
	if err = dao.kvstore.PutIfNotExists(blockNS, totalExecutionsKey, make([]byte, 8)); err != nil {
		return errors.Wrap(err, "failed to write initial value for total executions")
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
func (dao *blockDAO) getBlock(hash hash.Hash32B) (*Block, error) {
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

// getReceiptByExecutionHash returns the receipt by execution hash
func (dao *blockDAO) getReceiptByExecutionHash(h hash.Hash32B) (*Receipt, error) {
	value, err := dao.kvstore.Get(blockExecutionReceiptMappingNS, h[:])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get receipt for execution %x", h[:])
	}
	r := Receipt{}
	if err := r.Deserialize(value); err != nil {
		return nil, err
	}
	return &r, nil
}

// putBlock puts a block
func (dao *blockDAO) putBlock(blk *Block) error {
	batch := dao.kvstore.Batch()

	height := byteutil.Uint64ToBytes(blk.Height())

	serialized, err := blk.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize block")
	}
	hash := blk.HashBlock()
	batch.PutIfNotExists(blockNS, hash[:], serialized, "failed to put block")

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

	value, err = dao.kvstore.Get(blockNS, totalTransfersKey)
	if err != nil {
		return errors.Wrap(err, "failed to get total transfers")
	}
	totalTransfers := enc.MachineEndian.Uint64(value)
	totalTransfers += uint64(len(blk.Transfers))
	totalTransfersBytes := byteutil.Uint64ToBytes(totalTransfers)
	batch.Put(blockNS, totalTransfersKey, totalTransfersBytes, "failed to put total transfers")

	value, err = dao.kvstore.Get(blockNS, totalVotesKey)
	if err != nil {
		return errors.Wrap(err, "failed to get total votes")
	}
	totalVotes := enc.MachineEndian.Uint64(value)
	totalVotes += uint64(len(blk.Votes))
	totalVotesBytes := byteutil.Uint64ToBytes(totalVotes)
	batch.Put(blockNS, totalVotesKey, totalVotesBytes, "failed to put total votes")

	value, err = dao.kvstore.Get(blockNS, totalExecutionsKey)
	if err != nil {
		return errors.Wrap(err, "failed to get total executions")
	}
	totalExecutions := enc.MachineEndian.Uint64(value)
	totalExecutions += uint64(len(blk.Executions))
	totalExecutionsBytes := byteutil.Uint64ToBytes(totalExecutions)
	batch.Put(blockNS, totalExecutionsKey, totalExecutionsBytes, "failed to put total executions")

	// map Transfer hash to block hash
	for _, transfer := range blk.Transfers {
		transferHash := transfer.Hash()
		hashKey := append(transferPrefix, transferHash[:]...)
		batch.Put(blockTransferBlockMappingNS, hashKey, hash[:], "failed to put transfer hash %x", transferHash)
	}

	// map Vote hash to block hash
	for _, vote := range blk.Votes {
		voteHash := vote.Hash()
		hashKey := append(votePrefix, voteHash[:]...)
		batch.Put(blockVoteBlockMappingNS, hashKey, hash[:], "failed to put vote hash %x", voteHash)
	}

	// map execution hash to block hash
	for _, execution := range blk.Executions {
		executionHash := execution.Hash()
		hashKey := append(executionPrefix, executionHash[:]...)
		batch.Put(blockExecutionBlockMappingNS, hashKey, hash[:], "failed to put execution hash %x", executionHash)
	}

	if err = putTransfers(dao, blk, batch); err != nil {
		return err
	}

	if err = putVotes(dao, blk, batch); err != nil {
		return err
	}

	if err = putExecutions(dao, blk, batch); err != nil {
		return err
	}

	if err = putReceipts(dao, blk, batch); err != nil {
		return err
	}

	return batch.Commit()
}

// putTransfers stores transfer information into db
func putTransfers(dao *blockDAO, blk *Block, batch db.KVStoreBatch) error {
	senderDelta := map[string]uint64{}
	recipientDelta := map[string]uint64{}

	for _, transfer := range blk.Transfers {
		transferHash := transfer.Hash()

		// get transfers count for sender
		senderTransferCount, err := dao.getTransferCountBySenderAddress(transfer.Sender)
		if err != nil {
			return errors.Wrapf(err, "for sender %x", transfer.Sender)
		}
		if delta, ok := senderDelta[transfer.Sender]; ok {
			senderTransferCount += delta
			senderDelta[transfer.Sender] = senderDelta[transfer.Sender] + 1
		} else {
			senderDelta[transfer.Sender] = 1
		}

		// put new transfer to sender
		senderKey := append(transferFromPrefix, transfer.Sender...)
		senderKey = append(senderKey, byteutil.Uint64ToBytes(senderTransferCount)...)
		batch.PutIfNotExists(blockAddressTransferMappingNS, senderKey, transferHash[:], "failed to put transfer hash %x for sender %x",
			transfer.Hash(), transfer.Sender)

		// update sender transfers count
		senderTransferCountKey := append(transferFromPrefix, transfer.Sender...)
		batch.Put(blockAddressTransferCountMappingNS, senderTransferCountKey,
			byteutil.Uint64ToBytes(senderTransferCount+1), "failed to bump transfer count %x for sender %x",
			transfer.Hash(), transfer.Sender)

		// get transfers count for recipient
		recipientTransferCount, err := dao.getTransferCountByRecipientAddress(transfer.Recipient)
		if err != nil {
			return errors.Wrapf(err, "for recipient %x", transfer.Recipient)
		}
		if delta, ok := recipientDelta[transfer.Recipient]; ok {
			recipientTransferCount += delta
			recipientDelta[transfer.Recipient] = recipientDelta[transfer.Recipient] + 1
		} else {
			recipientDelta[transfer.Recipient] = 1
		}

		// put new transfer to recipient
		recipientKey := append(transferToPrefix, transfer.Recipient...)
		recipientKey = append(recipientKey, byteutil.Uint64ToBytes(recipientTransferCount)...)
		batch.PutIfNotExists(blockAddressTransferMappingNS, recipientKey, transferHash[:], "failed to put transfer hash %x for recipient %x",
			transfer.Hash(), transfer.Recipient)

		// update recipient transfers count
		recipientTransferCountKey := append(transferToPrefix, transfer.Recipient...)
		batch.Put(blockAddressTransferCountMappingNS, recipientTransferCountKey,
			byteutil.Uint64ToBytes(recipientTransferCount+1), "failed to bump transfer count %x for recipient %x",
			transfer.Hash(), transfer.Recipient)
	}

	return nil
}

// putVotes stores vote information into db
func putVotes(dao *blockDAO, blk *Block, batch db.KVStoreBatch) error {
	senderDelta := map[string]uint64{}
	recipientDelta := map[string]uint64{}

	for _, vote := range blk.Votes {
		voteHash := vote.Hash()

		pbVote := vote.GetVote()
		Sender := pbVote.VoterAddress
		Recipient := pbVote.VoteeAddress

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
		batch.PutIfNotExists(blockAddressVoteMappingNS, senderKey, voteHash[:], "failed to put vote hash %x for sender %x",
			voteHash, Sender)

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
		batch.PutIfNotExists(blockAddressVoteMappingNS, recipientKey, voteHash[:], "failed to put vote hash %x for recipient %x",
			voteHash, Recipient)

		// update recipient votes count
		recipientVoteCountKey := append(voteToPrefix, Recipient...)
		batch.Put(blockAddressVoteCountMappingNS, recipientVoteCountKey,
			byteutil.Uint64ToBytes(recipientVoteCount+1), "failed to bump vote count %x for recipient %x",
			voteHash, Recipient)
	}

	return nil
}

// putExecutions stores execution information into db
func putExecutions(dao *blockDAO, blk *Block, batch db.KVStoreBatch) error {
	executorDelta := map[string]uint64{}
	contractDelta := map[string]uint64{}

	for _, execution := range blk.Executions {
		executionHash := execution.Hash()

		// get execution count for executor
		executorExecutionCount, err := dao.getExecutionCountByExecutorAddress(execution.Executor)
		if err != nil {
			return errors.Wrapf(err, "for executor %x", execution.Executor)
		}
		if delta, ok := executorDelta[execution.Executor]; ok {
			executorExecutionCount += delta
			executorDelta[execution.Executor] = executorDelta[execution.Executor] + 1
		} else {
			executorDelta[execution.Executor] = 1
		}

		// put new execution to executor
		executorKey := append(executionFromPrefix, execution.Executor...)
		executorKey = append(executorKey, byteutil.Uint64ToBytes(executorExecutionCount)...)
		batch.PutIfNotExists(blockAddressExecutionMappingNS, executorKey, executionHash[:], "failed to put execution hash %x for executor %x",
			execution.Hash(), execution.Executor)

		// update executor executions count
		executorExecutionCountKey := append(executionFromPrefix, execution.Executor...)
		batch.Put(blockAddressExecutionCountMappingNS, executorExecutionCountKey,
			byteutil.Uint64ToBytes(executorExecutionCount+1), "failed to bump execution count %x for executor %x",
			execution.Hash(), execution.Executor)

		// get execution count for contract
		contractExecutionCount, err := dao.getExecutionCountByContractAddress(execution.Contract)
		if err != nil {
			return errors.Wrapf(err, "for contract %x", execution.Contract)
		}
		if delta, ok := contractDelta[execution.Contract]; ok {
			contractExecutionCount += delta
			contractDelta[execution.Contract] = contractDelta[execution.Contract] + 1
		} else {
			contractDelta[execution.Contract] = 1
		}

		// put new execution to contract
		contractKey := append(executionToPrefix, execution.Contract...)
		contractKey = append(contractKey, byteutil.Uint64ToBytes(contractExecutionCount)...)
		batch.PutIfNotExists(blockAddressExecutionMappingNS, contractKey, executionHash[:], "failed to put execution hash %x for contract %x",
			execution.Hash(), execution.Contract)

		// update contract executions count
		contractExecutionCountKey := append(executionToPrefix, execution.Contract...)
		batch.Put(blockAddressExecutionCountMappingNS, contractExecutionCountKey,
			byteutil.Uint64ToBytes(contractExecutionCount+1), "failed to bump execution count %x for contract %x",
			execution.Hash(), execution.Contract)
	}
	return nil
}

// putReceipts store receipt into db
func putReceipts(dao *blockDAO, blk *Block, batch db.KVStoreBatch) error {
	for _, r := range blk.receipts {
		v, err := r.Serialize()
		if err != nil {
			return errors.Wrapf(err, "failed to serialize receipt %x", r.Hash[:])
		}
		batch.Put(blockExecutionReceiptMappingNS, r.Hash[:], v[:], "failed to put receipt for execution %x", r.Hash[:])
	}
	return nil
}
