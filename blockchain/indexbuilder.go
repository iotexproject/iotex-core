// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"strconv"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

var batchSizeMtc = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "iotex_indexer_batch_size",
		Help: "Indexer batch size",
	},
	[]string{},
)

func init() {
	prometheus.MustRegister(batchSizeMtc)
}

// IndexBuilder defines the index builder
type IndexBuilder struct {
	store        db.KVStore
	pendingBlks  chan *block.Block
	cancelChan   chan interface{}
	timerFactory *prometheustimer.TimerFactory
}

// NewIndexBuilder instantiates an index builder
func NewIndexBuilder(chain Blockchain) (*IndexBuilder, error) {
	bc, ok := chain.(*blockchain)
	if !ok {
		log.S().Panic("unexpected blockchain implementation")
	}
	timerFactory, err := prometheustimer.New(
		"iotex_indexer_batch_time",
		"Indexer batch time",
		[]string{"topic", "chainID"},
		[]string{"default", strconv.FormatUint(uint64(bc.ChainID()), 10)},
	)
	if err != nil {
		return nil, err
	}
	return &IndexBuilder{
		store:        bc.dao.kvstore,
		pendingBlks:  make(chan *block.Block, 64), // Actually 1 should be enough
		cancelChan:   make(chan interface{}),
		timerFactory: timerFactory,
	}, nil
}

// Start starts the index builder
func (ib *IndexBuilder) Start(_ context.Context) error {
	go func() {
		for {
			select {
			case <-ib.cancelChan:
				return
			case blk := <-ib.pendingBlks:
				timer := ib.timerFactory.NewTimer("indexBlock")
				batch := db.NewBatch()
				if err := indexBlock(ib.store, blk, batch); err != nil {
					log.L().Info(
						"Error when indexing the block",
						zap.Uint64("height", blk.Height()),
						zap.Error(err),
					)
				}
				// index receipts
				if err := putReceipts(blk.Height(), blk.Receipts, batch); err != nil {
					log.L().Info(
						"Error when indexing the block",
						zap.Uint64("height", blk.Height()),
						zap.Error(err),
					)
				}
				batchSizeMtc.WithLabelValues().Set(float64(batch.Size()))
				if err := ib.store.Commit(batch); err != nil {
					log.L().Info(
						"Error when indexing the block",
						zap.Uint64("height", blk.Height()),
						zap.Error(err),
					)
				}
				timer.End()
			}
		}
	}()
	return nil
}

// Stop stops the index builder
func (ib *IndexBuilder) Stop(_ context.Context) error {
	close(ib.cancelChan)
	return nil
}

// HandleBlock handles the block and create the indices for the actions and receipts in it
func (ib *IndexBuilder) HandleBlock(blk *block.Block) error {
	ib.pendingBlks <- blk
	return nil
}

func indexBlock(store db.KVStore, blk *block.Block, batch db.KVStoreBatch) error {
	hash := blk.HashBlock()

	// TODO: To be deprecated
	// only build Tsf/Vote/Execution index if enable explorer
	transfers, votes, executions := action.ClassifyActions(blk.Actions)
	value, err := store.Get(blockNS, totalTransfersKey)
	if err != nil {
		return errors.Wrap(err, "failed to get total transfers")
	}
	totalTransfers := enc.MachineEndian.Uint64(value)
	totalTransfers += uint64(len(transfers))
	totalTransfersBytes := byteutil.Uint64ToBytes(totalTransfers)
	batch.Put(blockNS, totalTransfersKey, totalTransfersBytes, "failed to put total transfers")

	value, err = store.Get(blockNS, totalVotesKey)
	if err != nil {
		return errors.Wrap(err, "failed to get total votes")
	}
	totalVotes := enc.MachineEndian.Uint64(value)
	totalVotes += uint64(len(votes))
	totalVotesBytes := byteutil.Uint64ToBytes(totalVotes)
	batch.Put(blockNS, totalVotesKey, totalVotesBytes, "failed to put total votes")

	value, err = store.Get(blockNS, totalExecutionsKey)
	if err != nil {
		return errors.Wrap(err, "failed to get total executions")
	}
	totalExecutions := enc.MachineEndian.Uint64(value)
	totalExecutions += uint64(len(executions))
	totalExecutionsBytes := byteutil.Uint64ToBytes(totalExecutions)
	batch.Put(blockNS, totalExecutionsKey, totalExecutionsBytes, "failed to put total executions")

	value, err = store.Get(blockNS, totalActionsKey)
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
	if err := putTransfers(store, blk, batch); err != nil {
		return err
	}

	// TODO: To be deprecated
	if err := putVotes(store, blk, batch); err != nil {
		return err
	}

	// TODO: To be deprecated
	if err := putExecutions(store, blk, batch); err != nil {
		return err
	}

	return putActions(store, blk, batch)
}

// TODO: To be deprecated
// putTransfers stores transfer information into db
func putTransfers(store db.KVStore, blk *block.Block, batch db.KVStoreBatch) error {
	senderDelta := map[string]uint64{}
	recipientDelta := map[string]uint64{}

	transfers, _, _ := action.ClassifyActions(blk.Actions)
	for _, transfer := range transfers {
		transferHash := transfer.Hash()

		callerPKHash := keypair.HashPubKey(transfer.SrcPubkey())
		callerAddr, err := address.FromBytes(callerPKHash[:])
		if err != nil {
			return err
		}
		callerAddrStr := callerAddr.String()

		// get transfers count for sender
		senderTransferCount, err := getTransferCountBySenderAddress(store, callerAddrStr)
		if err != nil {
			return errors.Wrapf(err, "for sender %x", callerAddrStr)
		}
		if delta, ok := senderDelta[callerAddrStr]; ok {
			senderTransferCount += delta
			senderDelta[callerAddrStr]++
		} else {
			senderDelta[callerAddrStr] = 1
		}

		// put new transfer to sender
		senderKey := append(transferFromPrefix, callerAddrStr...)
		senderKey = append(senderKey, byteutil.Uint64ToBytes(senderTransferCount)...)
		batch.Put(blockAddressTransferMappingNS, senderKey, transferHash[:],
			"failed to put transfer hash %x for sender %x", transfer.Hash(), callerAddrStr)

		// update sender transfers count
		senderTransferCountKey := append(transferFromPrefix, callerAddrStr...)
		batch.Put(blockAddressTransferCountMappingNS, senderTransferCountKey,
			byteutil.Uint64ToBytes(senderTransferCount+1), "failed to bump transfer count %x for sender %x",
			transfer.Hash(), callerAddrStr)

		// get transfers count for recipient
		recipientTransferCount, err := getTransferCountByRecipientAddress(store, transfer.Recipient())
		if err != nil {
			return errors.Wrapf(err, "for recipient %x", transfer.Recipient())
		}
		if delta, ok := recipientDelta[transfer.Recipient()]; ok {
			recipientTransferCount += delta
			recipientDelta[transfer.Recipient()]++
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
func putVotes(store db.KVStore, blk *block.Block, batch db.KVStoreBatch) error {
	senderDelta := map[string]uint64{}
	recipientDelta := map[string]uint64{}

	for _, selp := range blk.Actions {
		vote, ok := selp.Action().(*action.Vote)
		if !ok {
			continue
		}
		voteHash := selp.Hash()
		callerPKHash := keypair.HashPubKey(vote.SrcPubkey())
		callerAddr, err := address.FromBytes(callerPKHash[:])
		if err != nil {
			return err
		}
		callerAddrStr := callerAddr.String()
		Recipient := vote.Votee()

		// get votes count for sender
		senderVoteCount, err := getVoteCountBySenderAddress(store, callerAddrStr)
		if err != nil {
			return errors.Wrapf(err, "for sender %x", callerAddrStr)
		}
		if delta, ok := senderDelta[callerAddrStr]; ok {
			senderVoteCount += delta
			senderDelta[callerAddrStr]++
		} else {
			senderDelta[callerAddrStr] = 1
		}

		// put new vote to sender
		senderKey := append(voteFromPrefix, callerAddrStr...)
		senderKey = append(senderKey, byteutil.Uint64ToBytes(senderVoteCount)...)
		batch.Put(blockAddressVoteMappingNS, senderKey, voteHash[:],
			"failed to put vote hash %x for sender %x", voteHash, callerAddrStr)

		// update sender votes count
		senderVoteCountKey := append(voteFromPrefix, callerAddrStr...)
		batch.Put(blockAddressVoteCountMappingNS, senderVoteCountKey,
			byteutil.Uint64ToBytes(senderVoteCount+1), "failed to bump vote count %x for sender %x",
			voteHash, callerAddrStr)

		// get votes count for recipient
		recipientVoteCount, err := getVoteCountByRecipientAddress(store, Recipient)
		if err != nil {
			return errors.Wrapf(err, "for recipient %x", Recipient)
		}
		if delta, ok := recipientDelta[Recipient]; ok {
			recipientVoteCount += delta
			recipientDelta[Recipient]++
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
func putExecutions(store db.KVStore, blk *block.Block, batch db.KVStoreBatch) error {
	executorDelta := map[string]uint64{}
	contractDelta := map[string]uint64{}

	_, _, executions := action.ClassifyActions(blk.Actions)
	for _, execution := range executions {
		executionHash := execution.Hash()
		callerPKHash := keypair.HashPubKey(execution.SrcPubkey())
		callerAddr, err := address.FromBytes(callerPKHash[:])
		if err != nil {
			return err
		}
		callerAddrStr := callerAddr.String()

		// get execution count for executor
		executorExecutionCount, err := getExecutionCountByExecutorAddress(store, callerAddrStr)
		if err != nil {
			return errors.Wrapf(err, "for executor %x", callerAddrStr)
		}
		if delta, ok := executorDelta[callerAddrStr]; ok {
			executorExecutionCount += delta
			executorDelta[callerAddrStr]++
		} else {
			executorDelta[callerAddrStr] = 1
		}

		// put new execution to executor
		executorKey := append(executionFromPrefix, callerAddrStr...)
		executorKey = append(executorKey, byteutil.Uint64ToBytes(executorExecutionCount)...)
		batch.Put(blockAddressExecutionMappingNS, executorKey, executionHash[:],
			"failed to put execution hash %x for executor %x", execution.Hash(), callerAddrStr)

		// update executor executions count
		executorExecutionCountKey := append(executionFromPrefix, callerAddrStr...)
		batch.Put(blockAddressExecutionCountMappingNS, executorExecutionCountKey,
			byteutil.Uint64ToBytes(executorExecutionCount+1),
			"failed to bump execution count %x for executor %x", execution.Hash(), callerAddrStr)

		// get execution count for contract
		contractExecutionCount, err := getExecutionCountByContractAddress(store, execution.Contract())
		if err != nil {
			return errors.Wrapf(err, "for contract %x", execution.Contract())
		}
		if delta, ok := contractDelta[execution.Contract()]; ok {
			contractExecutionCount += delta
			contractDelta[execution.Contract()]++
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

func putActions(store db.KVStore, blk *block.Block, batch db.KVStoreBatch) error {
	senderDelta := make(map[string]uint64)
	recipientDelta := make(map[string]uint64)

	for _, selp := range blk.Actions {
		// TODO: before we deprecate separate index handling for transfer/vote/execution, we should avoid index it again
		// in the general way
		switch selp.Action().(type) {
		case *action.Transfer:
			continue
		case *action.Vote:
			continue
		case *action.Execution:
			continue
		}
		actHash := selp.Hash()
		callerPKHash := keypair.HashPubKey(selp.SrcPubkey())
		callerAddr, err := address.FromBytes(callerPKHash[:])
		if err != nil {
			return err
		}
		callerAddrStr := callerAddr.String()

		// get action count for sender
		senderActionCount, err := getActionCountBySenderAddress(store, callerAddrStr)
		if err != nil {
			return errors.Wrapf(err, "for sender %s", callerAddrStr)
		}
		if delta, ok := senderDelta[callerAddrStr]; ok {
			senderActionCount += delta
			senderDelta[callerAddrStr]++
		} else {
			senderDelta[callerAddrStr] = 1
		}

		// put new action to sender
		senderKey := append(actionFromPrefix, callerAddrStr...)
		senderKey = append(senderKey, byteutil.Uint64ToBytes(senderActionCount)...)
		batch.Put(blockAddressActionMappingNS, senderKey, actHash[:],
			"failed to put action hash %x for sender %s", actHash, callerAddrStr)

		// update sender action count
		senderActionCountKey := append(actionFromPrefix, callerAddrStr...)
		batch.Put(blockAddressActionCountMappingNS, senderActionCountKey,
			byteutil.Uint64ToBytes(senderActionCount+1),
			"failed to bump action count %x for sender %s", actHash, callerAddrStr)

		// get action count for recipient
		recipientActionCount, err := getActionCountByRecipientAddress(store, selp.DstAddr())
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
func putReceipts(blkHeight uint64, blkReceipts []*action.Receipt, batch db.KVStoreBatch) error {
	if blkReceipts == nil {
		return nil
	}
	var heightBytes [8]byte
	enc.MachineEndian.PutUint64(heightBytes[:], blkHeight)
	for _, r := range blkReceipts {
		batch.Put(
			blockActionReceiptMappingNS,
			r.ActHash[:],
			heightBytes[:],
			"Failed to put receipt index for action %x",
			r.ActHash[:],
		)
	}
	return nil
}

// TODO: To be deprecated
func getBlockHashByVoteHash(store db.KVStore, h hash.Hash256) (hash.Hash256, error) {
	blkHash := hash.ZeroHash256
	key := append(votePrefix, h[:]...)
	value, err := store.Get(blockVoteBlockMappingNS, key)
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
func getBlockHashByTransferHash(store db.KVStore, h hash.Hash256) (hash.Hash256, error) {
	blkHash := hash.ZeroHash256
	key := append(transferPrefix, h[:]...)
	value, err := store.Get(blockTransferBlockMappingNS, key)
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
func getBlockHashByExecutionHash(store db.KVStore, h hash.Hash256) (hash.Hash256, error) {
	blkHash := hash.ZeroHash256
	key := append(executionPrefix, h[:]...)
	value, err := store.Get(blockExecutionBlockMappingNS, key)
	if err != nil {
		return blkHash, errors.Wrapf(err, "failed to get execution %x", h)
	}
	if len(value) == 0 {
		return blkHash, errors.Wrapf(db.ErrNotExist, "execution %x missing", h)
	}
	copy(blkHash[:], value)
	return blkHash, nil
}

func getBlockHashByActionHash(store db.KVStore, h hash.Hash256) (hash.Hash256, error) {
	blkHash := hash.ZeroHash256
	key := append(actionPrefix, h[:]...)
	value, err := store.Get(blockActionBlockMappingNS, key)
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
func getTransfersBySenderAddress(store db.KVStore, address string) ([]hash.Hash256, error) {
	// get transfers count for sender
	senderTransferCount, err := getTransferCountBySenderAddress(store, address)
	if err != nil {
		return nil, errors.Wrapf(err, "for sender %x", address)
	}

	res, getTransfersErr := getTransfersByAddress(store, address, senderTransferCount, transferFromPrefix)
	if getTransfersErr != nil {
		return nil, getTransfersErr
	}

	return res, nil
}

// TODO: To be deprecated
// getTransferCountBySenderAddress returns transfer count by sender address
func getTransferCountBySenderAddress(store db.KVStore, address string) (uint64, error) {
	senderTransferCountKey := append(transferFromPrefix, address...)
	value, err := store.Get(blockAddressTransferCountMappingNS, senderTransferCountKey)
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
func getTransfersByRecipientAddress(store db.KVStore, address string) ([]hash.Hash256, error) {
	// get transfers count for recipient
	recipientTransferCount, getCountErr := getTransferCountByRecipientAddress(store, address)
	if getCountErr != nil {
		return nil, errors.Wrapf(getCountErr, "for recipient %x", address)
	}

	res, getTransfersErr := getTransfersByAddress(store, address, recipientTransferCount, transferToPrefix)
	if getTransfersErr != nil {
		return nil, getTransfersErr
	}

	return res, nil
}

// TODO: To be deprecated
// getTransfersByAddress returns transfers by address
func getTransfersByAddress(store db.KVStore, address string, count uint64, keyPrefix []byte) ([]hash.Hash256, error) {
	var res []hash.Hash256

	for i := uint64(0); i < count; i++ {
		// put new transfer to recipient
		key := append(keyPrefix, address...)
		key = append(key, byteutil.Uint64ToBytes(i)...)
		value, err := store.Get(blockAddressTransferMappingNS, key)
		if err != nil {
			return res, errors.Wrapf(err, "failed to get transfer for index %x", i)
		}
		if len(value) == 0 {
			return res, errors.Wrapf(db.ErrNotExist, "transfer for index %x missing", i)
		}
		transferHash := hash.ZeroHash256
		copy(transferHash[:], value)
		res = append(res, transferHash)
	}

	return res, nil
}

// TODO: To be deprecated
// getTransferCountByRecipientAddress returns transfer count by recipient address
func getTransferCountByRecipientAddress(store db.KVStore, address string) (uint64, error) {
	recipientTransferCountKey := append(transferToPrefix, address...)
	value, err := store.Get(blockAddressTransferCountMappingNS, recipientTransferCountKey)
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
func getVotesBySenderAddress(store db.KVStore, address string) ([]hash.Hash256, error) {
	senderVoteCount, err := getVoteCountBySenderAddress(store, address)
	if err != nil {
		return nil, errors.Wrapf(err, "to get votecount for sender %x", address)
	}

	res, err := getVotesByAddress(store, address, senderVoteCount, voteFromPrefix)
	if err != nil {
		return nil, errors.Wrapf(err, "to get votes for sender %x", address)
	}

	return res, nil
}

// TODO: To be deprecated
// getVoteCountBySenderAddress returns vote count by sender address
func getVoteCountBySenderAddress(store db.KVStore, address string) (uint64, error) {
	senderVoteCountKey := append(voteFromPrefix, address...)
	value, err := store.Get(blockAddressVoteCountMappingNS, senderVoteCountKey)
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
func getVotesByRecipientAddress(store db.KVStore, address string) ([]hash.Hash256, error) {
	recipientVoteCount, err := getVoteCountByRecipientAddress(store, address)
	if err != nil {
		return nil, errors.Wrapf(err, "to get votecount for recipient %x", address)
	}

	res, err := getVotesByAddress(store, address, recipientVoteCount, voteToPrefix)
	if err != nil {
		return nil, errors.Wrapf(err, "to get votes for recipient %x", address)
	}

	return res, nil
}

// TODO: To be deprecated
// getVotesByAddress returns votes by address
func getVotesByAddress(store db.KVStore, address string, count uint64, keyPrefix []byte) ([]hash.Hash256, error) {
	var res []hash.Hash256

	for i := uint64(0); i < count; i++ {
		// put new vote to recipient
		key := append(keyPrefix, address...)
		key = append(key, byteutil.Uint64ToBytes(i)...)
		value, err := store.Get(blockAddressVoteMappingNS, key)
		if err != nil {
			return res, errors.Wrapf(err, "failed to get vote for index %x", i)
		}
		if len(value) == 0 {
			return res, errors.Wrapf(db.ErrNotExist, "vote for index %x missing", i)
		}
		voteHash := hash.ZeroHash256
		copy(voteHash[:], value)
		res = append(res, voteHash)
	}

	return res, nil
}

// TODO: To be deprecated
// getVoteCountByRecipientAddress returns vote count by recipient address
func getVoteCountByRecipientAddress(store db.KVStore, address string) (uint64, error) {
	recipientVoteCountKey := append(voteToPrefix, address...)
	value, err := store.Get(blockAddressVoteCountMappingNS, recipientVoteCountKey)
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
func getExecutionsByExecutorAddress(store db.KVStore, address string) ([]hash.Hash256, error) {
	// get executions count for sender
	executorExecutionCount, err := getExecutionCountByExecutorAddress(store, address)
	if err != nil {
		return nil, errors.Wrapf(err, "for executor %x", address)
	}

	res, getExecutionsErr := getExecutionsByAddress(store, address, executorExecutionCount, executionFromPrefix)
	if getExecutionsErr != nil {
		return nil, getExecutionsErr
	}

	return res, nil
}

// TODO: To be deprecated
// getExecutionCountByExecutorAddress returns execution count by executor address
func getExecutionCountByExecutorAddress(store db.KVStore, address string) (uint64, error) {
	executorExecutionCountKey := append(executionFromPrefix, address...)
	value, err := store.Get(blockAddressExecutionCountMappingNS, executorExecutionCountKey)
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
func getExecutionsByContractAddress(store db.KVStore, address string) ([]hash.Hash256, error) {
	// get execution count for contract
	contractExecutionCount, getCountErr := getExecutionCountByContractAddress(store, address)
	if getCountErr != nil {
		return nil, errors.Wrapf(getCountErr, "for contract %x", address)
	}

	res, getExecutionsErr := getExecutionsByAddress(store, address, contractExecutionCount, executionToPrefix)
	if getExecutionsErr != nil {
		return nil, getExecutionsErr
	}

	return res, nil
}

// TODO: To be deprecated
// getExecutionsByAddress returns executions by address
func getExecutionsByAddress(store db.KVStore, address string, count uint64, keyPrefix []byte) ([]hash.Hash256, error) {
	var res []hash.Hash256

	for i := uint64(0); i < count; i++ {
		// put new execution to recipient
		key := append(keyPrefix, address...)
		key = append(key, byteutil.Uint64ToBytes(i)...)
		value, err := store.Get(blockAddressExecutionMappingNS, key)
		if err != nil {
			return res, errors.Wrapf(err, "failed to get execution for index %x", i)
		}
		if len(value) == 0 {
			return res, errors.Wrapf(db.ErrNotExist, "execution for index %x missing", i)
		}
		executionHash := hash.ZeroHash256
		copy(executionHash[:], value)
		res = append(res, executionHash)
	}

	return res, nil
}

// TODO: To be deprecated
// getExecutionCountByContractAddress returns execution count by contract address
func getExecutionCountByContractAddress(store db.KVStore, address string) (uint64, error) {
	contractExecutionCountKey := append(executionToPrefix, address...)
	value, err := store.Get(blockAddressExecutionCountMappingNS, contractExecutionCountKey)
	if err != nil {
		return 0, nil
	}
	if len(value) == 0 {
		return 0, errors.New("count of executions as contract is broken")
	}
	return enc.MachineEndian.Uint64(value), nil
}

// getActionCountBySenderAddress returns action count by sender address
func getActionCountBySenderAddress(store db.KVStore, address string) (uint64, error) {
	senderActionCountKey := append(actionFromPrefix, address...)
	value, err := store.Get(blockAddressActionCountMappingNS, senderActionCountKey)
	if err != nil {
		return 0, nil
	}
	if len(value) == 0 {
		return 0, errors.New("count of actions by sender is broken")
	}
	return enc.MachineEndian.Uint64(value), nil
}

// getActionsBySenderAddress returns actions for sender
func getActionsBySenderAddress(store db.KVStore, address string) ([]hash.Hash256, error) {
	// get action count for sender
	senderActionCount, err := getActionCountBySenderAddress(store, address)
	if err != nil {
		return nil, errors.Wrapf(err, "for sender %x", address)
	}

	res, err := getActionsByAddress(store, address, senderActionCount, actionFromPrefix)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// getActionsByRecipientAddress returns actions for recipient
func getActionsByRecipientAddress(store db.KVStore, address string) ([]hash.Hash256, error) {
	// get action count for recipient
	recipientActionCount, getCountErr := getActionCountByRecipientAddress(store, address)
	if getCountErr != nil {
		return nil, errors.Wrapf(getCountErr, "for recipient %x", address)
	}

	res, err := getActionsByAddress(store, address, recipientActionCount, actionToPrefix)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// getActionCountByRecipientAddress returns action count by recipient address
func getActionCountByRecipientAddress(store db.KVStore, address string) (uint64, error) {
	recipientActionCountKey := append(actionToPrefix, address...)
	value, err := store.Get(blockAddressActionCountMappingNS, recipientActionCountKey)
	if err != nil {
		return 0, nil
	}
	if len(value) == 0 {
		return 0, errors.New("count of actions by recipient is broken")
	}
	return enc.MachineEndian.Uint64(value), nil
}

// getActionsByAddress returns actions by address
func getActionsByAddress(store db.KVStore, address string, count uint64, keyPrefix []byte) ([]hash.Hash256, error) {
	var res []hash.Hash256

	for i := uint64(0); i < count; i++ {
		key := append(keyPrefix, address...)
		key = append(key, byteutil.Uint64ToBytes(i)...)
		value, err := store.Get(blockAddressActionMappingNS, key)
		if err != nil {
			return res, errors.Wrapf(err, "failed to get action for index %d", i)
		}
		if len(value) == 0 {
			return res, errors.Wrapf(db.ErrNotExist, "action for index %d missing", i)
		}
		actHash := hash.ZeroHash256
		copy(actHash[:], value)
		res = append(res, actHash)
	}

	return res, nil
}
