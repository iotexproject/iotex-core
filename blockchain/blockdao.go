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

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	iproto "github.com/iotexproject/iotex-core/proto"
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
	if _, err = dao.kvstore.Get(blockNS, topHeightKey); err != nil &&
		errors.Cause(err) == db.ErrNotExist {
		if err := dao.kvstore.Put(blockNS, topHeightKey, make([]byte, 8)); err != nil {
			return errors.Wrap(err, "failed to write initial value for top height")
		}
	}

	// TODO: To be deprecated
	// set init total transfer to be 0
	if _, err := dao.kvstore.Get(blockNS, totalTransfersKey); err != nil &&
		errors.Cause(err) == db.ErrNotExist {
		if err = dao.kvstore.Put(blockNS, totalTransfersKey, make([]byte, 8)); err != nil {
			return errors.Wrap(err, "failed to write initial value for total transfers")
		}
	}

	// TODO: To be deprecated
	// set init total vote to be 0
	if _, err := dao.kvstore.Get(blockNS, totalVotesKey); err != nil &&
		errors.Cause(err) == db.ErrNotExist {
		if err = dao.kvstore.Put(blockNS, totalVotesKey, make([]byte, 8)); err != nil {
			return errors.Wrap(err, "failed to write initial value for total votes")
		}
	}

	// TODO: To be deprecated
	// set init total executions to be 0
	if _, err := dao.kvstore.Get(blockNS, totalExecutionsKey); err != nil &&
		errors.Cause(err) == db.ErrNotExist {
		if err = dao.kvstore.Put(blockNS, totalExecutionsKey, make([]byte, 8)); err != nil {
			return errors.Wrap(err, "failed to write initial value for total executions")
		}
	}

	// set init total actions to be 0
	if _, err := dao.kvstore.Get(blockNS, totalActionsKey); err != nil &&
		errors.Cause(err) == db.ErrNotExist {
		if err = dao.kvstore.Put(blockNS, totalActionsKey, make([]byte, 8)); err != nil {
			return errors.Wrap(err, "failed to write initial value for total actions")
		}
	}

	return nil
}

// Stop stops block DAO.
func (dao *blockDAO) Stop(ctx context.Context) error { return dao.lifecycle.OnStop(ctx) }

// getBlockHash returns the block hash by height
func (dao *blockDAO) getBlockHash(height uint64) (hash.Hash256, error) {
	key := append(heightPrefix, byteutil.Uint64ToBytes(height)...)
	value, err := dao.kvstore.Get(blockHashHeightMappingNS, key)
	hash := hash.ZeroHash256
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
func (dao *blockDAO) getBlockHeight(hash hash.Hash256) (uint64, error) {
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
func (dao *blockDAO) getBlock(hash hash.Hash256) (*block.Block, error) {
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
func (dao *blockDAO) getReceiptByActionHash(h hash.Hash256) (*action.Receipt, error) {
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
		if r.ActHash == h {
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
	if err := indexBlock(dao.kvstore, blk, batch); err != nil {
		return err
	}
	return dao.kvstore.Commit(batch)
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
		if !dao.writeIndex {
			continue
		}
		batch.Put(
			blockActionReceiptMappingNS,
			r.ActHash[:],
			heightBytes[:],
			"Failed to put receipt index for action %x",
			r.ActHash[:],
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
		senderTransferCount, err := getTransferCountBySenderAddress(dao.kvstore, sender)
		if err != nil {
			return errors.Wrapf(err, "for sender %x", sender)
		}
		senderTransferCountKey := append(transferFromPrefix, sender...)
		senderCount[sender] = senderTransferCount - count
		batch.Put(blockAddressTransferCountMappingNS, senderTransferCountKey,
			byteutil.Uint64ToBytes(senderCount[sender]), "failed to update transfer count for sender %x", sender)
	}
	for recipient, count := range recipientCount {
		recipientTransferCount, err := getTransferCountByRecipientAddress(dao.kvstore, recipient)
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
		senderVoteCount, err := getVoteCountBySenderAddress(dao.kvstore, sender)
		if err != nil {
			return errors.Wrapf(err, "for sender %x", sender)
		}
		senderVoteCountKey := append(voteFromPrefix, sender...)
		senderCount[sender] = senderVoteCount - count
		batch.Put(blockAddressVoteCountMappingNS, senderVoteCountKey, byteutil.Uint64ToBytes(senderCount[sender]),
			"failed to update vote count for sender %x", sender)
	}
	for recipient, count := range recipientCount {
		recipientVoteCount, err := getVoteCountByRecipientAddress(dao.kvstore, recipient)
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
			senderDelta[Sender]++
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
			recipientDelta[Recipient]++
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
		executorExecutionCount, err := getExecutionCountByExecutorAddress(dao.kvstore, executor)
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
		contractExecutionCount, err := getExecutionCountByContractAddress(dao.kvstore, contract)
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
		batch.Delete(blockActionReceiptMappingNS, r.ActHash[:], "failed to delete receipt for action %x", r.ActHash[:])
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
		senderActionCount, err := getActionCountBySenderAddress(dao.kvstore, sender)
		if err != nil {
			return errors.Wrapf(err, "for sender %x", sender)
		}
		senderActionCountKey := append(actionFromPrefix, sender...)
		senderCount[sender] = senderActionCount - count
		batch.Put(blockAddressActionCountMappingNS, senderActionCountKey, byteutil.Uint64ToBytes(senderCount[sender]),
			"failed to update action count for sender %x", sender)
	}
	for recipient, count := range recipientCount {
		recipientActionCount, err := getActionCountByRecipientAddress(dao.kvstore, recipient)
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
