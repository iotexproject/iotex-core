// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"sync"

	"github.com/golang/groupcache/lru"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/compress"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

const (
	blockNS                          = "blk"
	blockHashHeightMappingNS         = "h2h"
	blockActionBlockMappingNS        = "a2b"
	blockActionReceiptMappingNS      = "a2r"
	blockAddressActionMappingNS      = "a2a"
	blockAddressActionCountMappingNS = "a2c"
	receiptsNS                       = "rpt"

	hashOffset = 12
)

var (
	topHeightKey     = []byte("th")
	totalActionsKey  = []byte("ta")
	hashPrefix       = []byte("ha.")
	heightPrefix     = []byte("he.")
	actionFromPrefix = []byte("fr.")
	actionToPrefix   = []byte("to.")
)

var (
	cacheMtc = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_blockdao_cache",
			Help: "IoTeX blockdao cache counter.",
		},
		[]string{"result"},
	)
)

type blockDAO struct {
	writeIndex    bool
	compressBlock bool
	kvstore       db.KVStore
	timerFactory  *prometheustimer.TimerFactory
	lifecycle     lifecycle.Lifecycle
	cache         *lru.Cache
	cacheMutex    sync.Mutex
}

// newBlockDAO instantiates a block DAO
func newBlockDAO(kvstore db.KVStore, writeIndex bool, compressBlock bool, maxCacheSize int) *blockDAO {
	blockDAO := &blockDAO{
		writeIndex:    writeIndex,
		compressBlock: compressBlock,
		kvstore:       kvstore,
	}
	if maxCacheSize > 0 {
		blockDAO.cache = lru.New(maxCacheSize)
	}
	timerFactory, err := prometheustimer.New(
		"iotex_block_dao_perf",
		"Performance of block DAO",
		[]string{"type"},
		[]string{"default"},
	)
	if err != nil {
		return nil
	}
	blockDAO.timerFactory = timerFactory
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
	if height == 0 {
		return hash.ZeroHash256, nil
	}
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
	if dao.cache != nil {
		dao.cacheMutex.Lock()
		cblk, ok := dao.cache.Get(hash)
		dao.cacheMutex.Unlock()
		if ok {
			cacheMtc.WithLabelValues("hit").Inc()
			return cblk.(*block.Block), nil
		}
		cacheMtc.WithLabelValues("miss").Inc()
	}
	value, err := dao.kvstore.Get(blockNS, hash[:])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block %x", hash)
	}
	if dao.compressBlock {
		timer := dao.timerFactory.NewTimer("decompress")
		value, err = compress.Decompress(value)
		timer.End()
		if err != nil {
			return nil, errors.Wrapf(err, "error when decompressing a block")
		}
	}
	if len(value) == 0 {
		return nil, errors.Wrapf(db.ErrNotExist, "block %x missing", hash)
	}
	blk := block.Block{}
	if err = blk.Deserialize(value); err != nil {
		return nil, errors.Wrap(err, "failed to deserialize block")
	}
	if dao.cache != nil {
		dao.cacheMutex.Lock()
		dao.cache.Add(hash, &blk)
		dao.cacheMutex.Unlock()
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
	heightBytes, err := dao.kvstore.Get(blockActionReceiptMappingNS, h[hashOffset:])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get receipt index for action %x", h)
	}
	receiptsBytes, err := dao.kvstore.Get(receiptsNS, heightBytes)
	if err != nil {
		height := enc.MachineEndian.Uint64(heightBytes)
		return nil, errors.Wrapf(err, "failed to get receipts of block %d", height)
	}
	receipts := iotextypes.Receipts{}
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
	if dao.compressBlock {
		timer := dao.timerFactory.NewTimer("compress")
		serialized, err = compress.Compress(serialized)
		timer.End()
		if err != nil {
			return errors.Wrapf(err, "error when compressing a block")
		}
	}
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
	receipts := iotextypes.Receipts{}
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
			r.ActHash[hashOffset:],
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

	// update total action count
	value, err := dao.kvstore.Get(blockNS, totalActionsKey)
	if err != nil {
		return errors.Wrap(err, "failed to get total actions")
	}
	totalActions := enc.MachineEndian.Uint64(value)
	totalActions -= uint64(len(blk.Actions))
	totalActionsBytes := byteutil.Uint64ToBytes(totalActions)
	batch.Put(blockNS, totalActionsKey, totalActionsBytes, "failed to put total actions")

	// Delete action hash -> block hash mapping
	for _, selp := range blk.Actions {
		actHash := selp.Hash()
		batch.Delete(blockActionBlockMappingNS, actHash[hashOffset:], "failed to delete actions f")
	}

	if err = deleteActions(dao, blk, batch); err != nil {
		return err
	}

	if err = deleteReceipts(blk, batch); err != nil {
		return err
	}

	if dao.cache != nil {
		dao.cacheMutex.Lock()
		dao.cache.Remove(hash)
		dao.cacheMutex.Unlock()
	}
	return dao.kvstore.Commit(batch)
}

// deleteReceipts deletes receipt information from db
func deleteReceipts(blk *block.Block, batch db.KVStoreBatch) error {
	for _, r := range blk.Receipts {
		batch.Delete(blockActionReceiptMappingNS, r.ActHash[hashOffset:], "failed to delete receipt for action %x", r.ActHash[:])
	}
	return nil
}

// deleteActions deletes action information from db
func deleteActions(dao *blockDAO, blk *block.Block, batch db.KVStoreBatch) error {
	// Firt get the total count of actions by sender and recipient respectively in the block
	senderCount := make(map[hash.Hash160]uint64)
	recipientCount := make(map[hash.Hash160]uint64)
	for _, selp := range blk.Actions {
		callerAddrBytes := hash.BytesToHash160(selp.SrcPubkey().Hash())
		senderCount[callerAddrBytes]++
		if dst, ok := selp.Destination(); ok && dst != "" {
			dstAddr, err := address.FromString(dst)
			if err != nil {
				return err
			}
			dstAddrBytes := hash.BytesToHash160(dstAddr.Bytes())
			recipientCount[dstAddrBytes]++
		}
	}
	// Roll back the status of address -> actionCount mapping to the preivous block
	for sender, count := range senderCount {
		senderActionCount, err := getActionCountBySenderAddress(dao.kvstore, sender)
		if err != nil {
			return errors.Wrapf(err, "for sender %x", sender)
		}
		senderActionCountKey := append(actionFromPrefix, sender[:]...)
		senderCount[sender] = senderActionCount - count
		batch.Put(blockAddressActionCountMappingNS, senderActionCountKey, byteutil.Uint64ToBytes(senderCount[sender]),
			"failed to update action count for sender %x", sender)
	}
	for recipient, count := range recipientCount {
		recipientActionCount, err := getActionCountByRecipientAddress(dao.kvstore, recipient)
		if err != nil {
			return errors.Wrapf(err, "for recipient %x", recipient)
		}
		recipientActionCountKey := append(actionToPrefix, recipient[:]...)
		recipientCount[recipient] = recipientActionCount - count
		batch.Put(blockAddressActionCountMappingNS, recipientActionCountKey,
			byteutil.Uint64ToBytes(recipientCount[recipient]), "failed to update action count for recipient %x",
			recipient)

	}

	senderDelta := map[hash.Hash160]uint64{}
	recipientDelta := map[hash.Hash160]uint64{}

	for _, selp := range blk.Actions {
		actHash := selp.Hash()
		callerAddrBytes := hash.BytesToHash160(selp.SrcPubkey().Hash())

		if delta, ok := senderDelta[callerAddrBytes]; ok {
			senderCount[callerAddrBytes] += delta
			senderDelta[callerAddrBytes]++
		} else {
			senderDelta[callerAddrBytes] = 1
		}

		// Delete new action from sender
		senderKey := append(actionFromPrefix, callerAddrBytes[:]...)
		senderKey = append(senderKey, byteutil.Uint64ToBytes(senderCount[callerAddrBytes])...)
		batch.Delete(blockAddressActionMappingNS, senderKey, "failed to delete action hash %x for sender %x",
			actHash, callerAddrBytes)

		dst, ok := selp.Destination()
		if !ok || dst == "" {
			continue
		}
		dstAddr, err := address.FromString(dst)
		if err != nil {
			return err
		}
		dstAddrBytes := hash.BytesToHash160(dstAddr.Bytes())
		if delta, ok := recipientDelta[dstAddrBytes]; ok {
			recipientCount[dstAddrBytes] += delta
			recipientDelta[dstAddrBytes]++
		} else {
			recipientDelta[dstAddrBytes] = 1
		}

		// Delete new action to recipient
		recipientKey := append(actionToPrefix, dstAddrBytes[:]...)
		recipientKey = append(recipientKey, byteutil.Uint64ToBytes(recipientCount[dstAddrBytes])...)
		batch.Delete(blockAddressActionMappingNS, recipientKey, "failed to delete action hash %x for recipient %x",
			actHash, dstAddrBytes)
	}

	return nil
}
