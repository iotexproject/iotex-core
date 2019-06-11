// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/cache"
	"github.com/iotexproject/iotex-core/pkg/compress"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const (
	blockNS                          = "blk"
	blockHashHeightMappingNS         = "h2h"
	blockActionBlockMappingNS        = "a2b"
	blockActionReceiptMappingNS      = "a2r"
	blockAddressActionMappingNS      = "a2a"
	blockAddressActionCountMappingNS = "a2c"
	blockHeaderNS                    = "bhr"
	blockBodyNS                      = "bbd"
	blockFooterNS                    = "bfr"
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
	headerCache   *cache.ThreadSafeLruCache
	bodyCache     *cache.ThreadSafeLruCache
	footerCache   *cache.ThreadSafeLruCache
}

// newBlockDAO instantiates a block DAO
func newBlockDAO(kvstore db.KVStore, writeIndex bool, compressBlock bool, maxCacheSize int) *blockDAO {
	blockDAO := &blockDAO{
		writeIndex:    writeIndex,
		compressBlock: compressBlock,
		kvstore:       kvstore,
	}
	if maxCacheSize > 0 {
		blockDAO.headerCache = cache.NewThreadSafeLruCache(maxCacheSize)
		blockDAO.bodyCache = cache.NewThreadSafeLruCache(maxCacheSize)
		blockDAO.footerCache = cache.NewThreadSafeLruCache(maxCacheSize)
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
	value, _ := dao.kvstore.Get(blockNS, totalActionsKey)
	totalActions := enc.MachineEndian.Uint64(value)
	if totalActions != 0 {
		return nil
	}
	return dao.countActions()
}
func (dao *blockDAO) countActions() error {
	totalActions := uint64(0)
	tipHeight, err := dao.getBlockchainHeight()
	if err != nil {
		return err
	}
	for i := uint64(1); i <= tipHeight; i++ {
		hash, err := dao.getBlockHash(i)
		if err != nil {
			return err
		}
		body, err := dao.body(hash)
		if err != nil {
			return err
		}
		totalActions += uint64(len(body.Actions))
		if i%1000 == 0 {
			zap.L().Info("Counting number of actions", zap.Uint64("height", i))
		}
	}
	totalActionsBytes := byteutil.Uint64ToBytes(totalActions)
	batch := db.NewBatch()
	batch.Put(blockNS, totalActionsKey, totalActionsBytes, "failed to put total actions")
	if err := dao.kvstore.Commit(batch); err != nil {
		return err
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
	header, err := dao.header(hash)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block header %x", hash)
	}
	body, err := dao.body(hash)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block body %x", hash)
	}
	footer, err := dao.footer(hash)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block footer %x", hash)
	}
	return &block.Block{
		Header: *header,
		Body:   *body,
		Footer: *footer,
	}, nil
}

// Header returns a block header
func (dao *blockDAO) Header(h hash.Hash256) (*block.Header, error) {
	return dao.header(h)
}

func (dao *blockDAO) header(h hash.Hash256) (*block.Header, error) {
	if dao.headerCache != nil {
		header, ok := dao.headerCache.Get(h)
		if ok {
			cacheMtc.WithLabelValues("hit_header").Inc()
			return header.(*block.Header), nil
		}
		cacheMtc.WithLabelValues("miss_header").Inc()
	}
	value, err := dao.kvstore.Get(blockHeaderNS, h[:])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block header %x", h)
	}
	if dao.compressBlock {
		timer := dao.timerFactory.NewTimer("decompress_header")
		value, err = compress.Decompress(value)
		timer.End()
		if err != nil {
			return nil, errors.Wrapf(err, "error when decompressing a block header %x", h)
		}
	}
	if len(value) == 0 {
		return nil, errors.Wrapf(db.ErrNotExist, "block header %x is missing", h)
	}
	header := &block.Header{}
	if err := header.Deserialize(value); err != nil {
		return nil, errors.Wrapf(err, "failed to deserialize block header %x", h)
	}
	if dao.headerCache != nil {
		dao.headerCache.Add(h, header)
	}

	return header, nil
}

// Body returns a block body
func (dao *blockDAO) Body(h hash.Hash256) (*block.Body, error) {
	return dao.body(h)
}

func (dao *blockDAO) body(h hash.Hash256) (*block.Body, error) {
	if dao.bodyCache != nil {
		body, ok := dao.bodyCache.Get(h)
		if ok {
			cacheMtc.WithLabelValues("hit_body").Inc()
			return body.(*block.Body), nil
		}
		cacheMtc.WithLabelValues("miss_body").Inc()
	}
	value, err := dao.kvstore.Get(blockBodyNS, h[:])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block body %x", h)
	}
	if dao.compressBlock {
		timer := dao.timerFactory.NewTimer("decompress_body")
		value, err = compress.Decompress(value)
		timer.End()
		if err != nil {
			return nil, errors.Wrapf(err, "error when decompressing a block body %x", h)
		}
	}
	if len(value) == 0 {
		return nil, errors.Wrapf(db.ErrNotExist, "block body %x is missing", h)
	}
	body := &block.Body{}
	if err := body.Deserialize(value); err != nil {
		return nil, errors.Wrapf(err, "failed to deserialize block body %x", h)
	}
	if dao.bodyCache != nil {
		dao.bodyCache.Add(h, body)
	}
	return body, nil
}

// Footer returns a block footer
func (dao *blockDAO) Footer(h hash.Hash256) (*block.Footer, error) {
	return dao.footer(h)
}

func (dao *blockDAO) footer(h hash.Hash256) (*block.Footer, error) {
	if dao.footerCache != nil {
		footer, ok := dao.footerCache.Get(h)
		if ok {
			cacheMtc.WithLabelValues("hit_footer").Inc()
			return footer.(*block.Footer), nil
		}
		cacheMtc.WithLabelValues("miss_footer").Inc()
	}
	value, err := dao.kvstore.Get(blockFooterNS, h[:])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block footer %x", h)
	}
	if dao.compressBlock {
		timer := dao.timerFactory.NewTimer("decompress_footer")
		value, err = compress.Decompress(value)
		timer.End()
		if err != nil {
			return nil, errors.Wrapf(err, "error when decompressing a block footer %x", h)
		}
	}
	if len(value) == 0 {
		return nil, errors.Wrapf(db.ErrNotExist, "block footer %x is missing", h)
	}
	footer := &block.Footer{}
	if err := footer.Deserialize(value); err != nil {
		return nil, errors.Wrapf(err, "failed to deserialize block footer %x", h)
	}
	if dao.footerCache != nil {
		dao.footerCache.Add(h, footer)
	}
	return footer, nil
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
		if r.ActionHash == h {
			return &r, nil
		}
	}
	return nil, errors.Errorf("receipt of action %x isn't found", h)
}

// putBlock puts a block
func (dao *blockDAO) putBlock(blk *block.Block) error {
	batch := db.NewBatch()

	height := byteutil.Uint64ToBytes(blk.Height())
	hash := blk.HashBlock()
	serHeader, err := blk.Header.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize block header")
	}
	serBody, err := blk.Body.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize block body")
	}
	serFooter, err := blk.Footer.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize block footer")
	}
	if dao.compressBlock {
		timer := dao.timerFactory.NewTimer("compress_header")
		serHeader, err = compress.Compress(serHeader)
		timer.End()
		if err != nil {
			return errors.Wrapf(err, "error when compressing a block header")
		}
		timer = dao.timerFactory.NewTimer("compress_body")
		serBody, err = compress.Compress(serBody)
		timer.End()
		if err != nil {
			return errors.Wrapf(err, "error when compressing a block body")
		}
		timer = dao.timerFactory.NewTimer("compress_footer")
		serFooter, err = compress.Compress(serFooter)
		timer.End()
		if err != nil {
			return errors.Wrapf(err, "error when compressing a block footer")
		}
	}
	batch.Put(blockHeaderNS, hash[:], serHeader, "failed to put block header")
	batch.Put(blockBodyNS, hash[:], serBody, "failed to put block body")
	batch.Put(blockFooterNS, hash[:], serFooter, "failed to put block footer")

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

	value, err = dao.kvstore.Get(blockNS, totalActionsKey)
	if err != nil {
		return errors.Wrap(err, "failed to get total actions")
	}
	totalActions := enc.MachineEndian.Uint64(value)
	totalActions += uint64(len(blk.Actions))
	totalActionsBytes := byteutil.Uint64ToBytes(totalActions)
	batch.Put(blockNS, totalActionsKey, totalActionsBytes, "failed to put total actions")

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
			r.ActionHash[hashOffset:],
			heightBytes[:],
			"Failed to put receipt index for action %x",
			r.ActionHash[:],
		)
	}
	receiptsBytes, err := proto.Marshal(&receipts)
	if err != nil {
		return err
	}
	batch.Put(receiptsNS, heightBytes[:], receiptsBytes, "Failed to put receipts of block %d", blkHeight)
	return dao.kvstore.Commit(batch)
}

// getReceipts gets receipts
func (dao *blockDAO) getReceipts(blkHeight uint64) ([]*action.Receipt, error) {
	var heightBytes [8]byte
	enc.MachineEndian.PutUint64(heightBytes[:], blkHeight)

	value, err := dao.kvstore.Get(receiptsNS, heightBytes[:])
	if err != nil {
		return nil, errors.Wrap(err, "failed to get receipts")
	}
	if len(value) == 0 {
		return nil, errors.Wrap(db.ErrNotExist, "block receipts missing")
	}
	receiptsPb := &iotextypes.Receipts{}
	if err := proto.Unmarshal(value, receiptsPb); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal block receipts")
	}
	var blockReceipts []*action.Receipt
	for _, receiptPb := range receiptsPb.Receipts {
		receipt := &action.Receipt{}
		receipt.ConvertFromReceiptPb(receiptPb)
		blockReceipts = append(blockReceipts, receipt)
	}
	return blockReceipts, nil
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
	batch.Delete(blockHeaderNS, hash[:], "failed to delete block")
	if dao.headerCache != nil {
		dao.headerCache.Remove(hash)
	}
	batch.Delete(blockBodyNS, hash[:], "failed to delete block")
	if dao.bodyCache != nil {
		dao.bodyCache.Remove(hash)
	}
	batch.Delete(blockFooterNS, hash[:], "failed to delete block")
	if dao.footerCache != nil {
		dao.footerCache.Remove(hash)
	}

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

	return dao.kvstore.Commit(batch)
}

// deleteReceipts deletes receipt information from db
func deleteReceipts(blk *block.Block, batch db.KVStoreBatch) error {
	for _, r := range blk.Receipts {
		batch.Delete(blockActionReceiptMappingNS, r.ActionHash[hashOffset:], "failed to delete receipt for action %x", r.ActionHash[:])
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
