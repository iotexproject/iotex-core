// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/compress"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const (
	blockNS       = "blk"
	blockHeaderNS = "bhr"
	blockBodyNS   = "bbd"
	blockFooterNS = "bfr"
	receiptsNS    = "rpt"
)

var (
	heightPrefix       = []byte("he.")
	heightToFileBucket = []byte("h2f")
	patternLen         = len("00000000.db")
	suffixLen          = len(".db")
)

type (
	// fileDAOLegacy handles chain db file before file split activation at Ithaca height
	fileDAOLegacy struct {
		compressBlock bool
		lifecycle     lifecycle.Lifecycle
		cfg           config.DB
		mutex         sync.RWMutex // for create new db file
		topIndex      atomic.Value
		htf           db.RangeIndex
		kvStore       db.KVStore
		kvStores      sync.Map //store like map[index]db.KVStore,index from 1...N
	}
)

func newFileDAOLegacy(kvStore db.KVStore, compressBlock bool, cfg config.DB) (FileDAO, error) {
	if kvStore == nil {
		return nil, errors.New("empty KVStore")
	}
	return &fileDAOLegacy{
		compressBlock: compressBlock,
		cfg:           cfg,
		kvStore:       kvStore,
	}, nil
}

func (fd *fileDAOLegacy) Start(ctx context.Context) error {
	if err := fd.kvStore.Start(ctx); err != nil {
		return err
	}

	// set init height value and transaction log flag
	if _, err := fd.kvStore.Get(blockNS, topHeightKey); err != nil &&
		errors.Cause(err) == db.ErrNotExist {
		zero8bytes := make([]byte, 8)
		if err := fd.kvStore.Put(blockNS, topHeightKey, zero8bytes); err != nil {
			return errors.Wrap(err, "failed to write initial value for top height")
		}
		if err := fd.kvStore.Put(systemLogNS, zero8bytes, []byte(systemLogNS)); err != nil {
			return errors.Wrap(err, "failed to write initial value for transaction log")
		}
	}

	// loop thru all legacy files
	model, dir := getFileNameAndDir(fd.cfg.DbPath)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}
	var maxN uint64
	for _, file := range files {
		name := file.Name()
		lens := len(name)
		if lens < patternLen || !strings.Contains(name, model) {
			continue
		}
		num := name[lens-patternLen : lens-suffixLen]
		n, err := strconv.Atoi(num)
		if err != nil {
			continue
		}
		if _, _, err := fd.openDB(uint64(n)); err != nil {
			return err
		}
		if uint64(n) > maxN {
			maxN = uint64(n)
		}
	}
	if maxN == 0 {
		maxN = 1
	}
	fd.topIndex.Store(maxN)
	return nil
}

func (fd *fileDAOLegacy) Stop(ctx context.Context) error {
	if err := fd.lifecycle.OnStop(ctx); err != nil {
		return err
	}
	return fd.kvStore.Stop(ctx)
}

func (fd *fileDAOLegacy) Height() (uint64, error) {
	value, err := fd.kvStore.Get(blockNS, topHeightKey)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get top height")
	}
	if len(value) != 8 {
		return 0, errors.Wrap(ErrDataCorruption, "blockchain height missing")
	}
	return enc.MachineEndian.Uint64(value), nil
}

func (fd *fileDAOLegacy) GetBlockHash(height uint64) (hash.Hash256, error) {
	h := hash.ZeroHash256
	if height == 0 {
		return h, nil
	}
	value, err := fd.kvStore.Get(blockHashHeightMappingNS, heightKey(height))
	if err != nil {
		return h, errors.Wrap(err, "failed to get block hash")
	}
	if len(h) != len(value) {
		return h, errors.Wrapf(err, "blockhash is broken with length = %d", len(value))
	}
	copy(h[:], value)
	return h, nil
}

func (fd *fileDAOLegacy) GetBlockHeight(h hash.Hash256) (uint64, error) {
	value, err := fd.kvStore.Get(blockHashHeightMappingNS, hashKey(h))
	if err != nil {
		return 0, errors.Wrap(err, "failed to get block height")
	}
	if len(value) != 8 {
		return 0, errors.Wrapf(ErrDataCorruption, "height missing for block with hash = %x", h)
	}
	return enc.MachineEndian.Uint64(value), nil
}

func (fd *fileDAOLegacy) GetBlock(h hash.Hash256) (*block.Block, error) {
	header, err := fd.header(h)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block header %x", h)
	}
	body, err := fd.body(h)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block body %x", h)
	}
	footer, err := fd.footer(h)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block footer %x", h)
	}
	return &block.Block{
		Header: *header,
		Body:   *body,
		Footer: *footer,
	}, nil
}

func (fd *fileDAOLegacy) GetBlockByHeight(height uint64) (*block.Block, error) {
	hash, err := fd.GetBlockHash(height)
	if err != nil {
		return nil, err
	}
	return fd.GetBlock(hash)
}

func (fd *fileDAOLegacy) GetReceipts(height uint64) ([]*action.Receipt, error) {
	kvStore, _, err := fd.getDBFromHeight(height)
	if err != nil {
		return nil, err
	}
	value, err := kvStore.Get(receiptsNS, byteutil.Uint64ToBytes(height))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get receipts of block %d", height)
	}
	if len(value) == 0 {
		return nil, errors.Wrap(ErrDataCorruption, "block receipts missing")
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

func (fd *fileDAOLegacy) header(h hash.Hash256) (*block.Header, error) {
	value, err := fd.getBlockValue(blockHeaderNS, h)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block header %x", h)
	}
	if fd.compressBlock {
		value, err = compress.Decompress(value)
		if err != nil {
			return nil, errors.Wrapf(err, "error when decompressing a block header %x", h)
		}
	}
	if len(value) == 0 {
		return nil, errors.Wrapf(ErrDataCorruption, "block header %x is missing", h)
	}

	header := &block.Header{}
	if err := header.Deserialize(value); err != nil {
		return nil, errors.Wrapf(err, "failed to deserialize block header %x", h)
	}
	return header, nil
}

func (fd *fileDAOLegacy) body(h hash.Hash256) (*block.Body, error) {
	value, err := fd.getBlockValue(blockBodyNS, h)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block body %x", h)
	}
	if fd.compressBlock {
		value, err = compress.Decompress(value)
		if err != nil {
			return nil, errors.Wrapf(err, "error when decompressing a block body %x", h)
		}
	}

	body := &block.Body{}
	if len(value) == 0 {
		// block body could be empty
		return body, nil
	}
	if err := body.Deserialize(value); err != nil {
		return nil, errors.Wrapf(err, "failed to deserialize block body %x", h)
	}
	return body, nil
}

func (fd *fileDAOLegacy) footer(h hash.Hash256) (*block.Footer, error) {
	value, err := fd.getBlockValue(blockFooterNS, h)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block footer %x", h)
	}
	if fd.compressBlock {
		value, err = compress.Decompress(value)
		if err != nil {
			return nil, errors.Wrapf(err, "error when decompressing a block footer %x", h)
		}
	}
	if len(value) == 0 {
		return nil, errors.Wrapf(ErrDataCorruption, "block footer %x is missing", h)
	}

	footer := &block.Footer{}
	if err := footer.Deserialize(value); err != nil {
		return nil, errors.Wrapf(err, "failed to deserialize block footer %x", h)
	}
	return footer, nil
}

func (fd *fileDAOLegacy) ContainsTransactionLog() bool {
	sys, err := fd.kvStore.Get(systemLogNS, make([]byte, 8))
	return err == nil && string(sys) == systemLogNS
}

func (fd *fileDAOLegacy) TransactionLogs(height uint64) (*iotextypes.TransactionLogs, error) {
	if !fd.ContainsTransactionLog() {
		return nil, ErrNotSupported
	}

	logsBytes, err := fd.kvStore.Get(systemLogNS, heightKey(height))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction log")
	}
	return block.DeserializeSystemLogPb(logsBytes)
}

func (fd *fileDAOLegacy) PutBlock(ctx context.Context, blk *block.Block) error {
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
	if fd.compressBlock {
		serHeader, err = compress.Compress(serHeader)
		if err != nil {
			return errors.Wrapf(err, "error when compressing a block header")
		}
		serBody, err = compress.Compress(serBody)
		if err != nil {
			return errors.Wrapf(err, "error when compressing a block body")
		}
		serFooter, err = compress.Compress(serFooter)
		if err != nil {
			return errors.Wrapf(err, "error when compressing a block footer")
		}
	}
	batchForBlock := batch.NewBatch()
	hash := blk.HashBlock()
	batchForBlock.Put(blockHeaderNS, hash[:], serHeader, "failed to put block header")
	batchForBlock.Put(blockBodyNS, hash[:], serBody, "failed to put block body")
	batchForBlock.Put(blockFooterNS, hash[:], serFooter, "failed to put block footer")
	blkHeight := blk.Height()
	heightKey := heightKey(blkHeight)
	if fd.ContainsTransactionLog() {
		if sysLog := blk.TransactionLog(); sysLog != nil {
			batchForBlock.Put(systemLogNS, heightKey, sysLog.Serialize(), "failed to put transaction log")
		}
	}
	kv, _, err := fd.getTopDB(blkHeight)
	if err != nil {
		return err
	}
	// write receipts
	if blk.Receipts != nil {
		receipts := iotextypes.Receipts{}
		for _, r := range blk.Receipts {
			receipts.Receipts = append(receipts.Receipts, r.ConvertToReceiptPb())
		}
		if receiptsBytes, err := proto.Marshal(&receipts); err == nil {
			batchForBlock.Put(receiptsNS, byteutil.Uint64ToBytes(blkHeight), receiptsBytes, "failed to put receipts")
		} else {
			log.L().Error("failed to serialize receipits for block", zap.Uint64("height", blkHeight))
		}
	}
	if err := kv.WriteBatch(batchForBlock); err != nil {
		return err
	}

	b := batch.NewBatch()
	heightValue := byteutil.Uint64ToBytes(blkHeight)
	hashKey := hashKey(hash)
	b.Put(blockHashHeightMappingNS, hashKey, heightValue, "failed to put hash -> height mapping")
	b.Put(blockHashHeightMappingNS, heightKey, hash[:], "failed to put height -> hash mapping")
	tipHeight, err := fd.kvStore.Get(blockNS, topHeightKey)
	if err != nil {
		return errors.Wrap(err, "failed to get top height")
	}
	if blkHeight > enc.MachineEndian.Uint64(tipHeight) {
		b.Put(blockNS, topHeightKey, heightValue, "failed to put top height")
		b.Put(blockNS, topHashKey, hash[:], "failed to put top hash")
	}
	return fd.kvStore.WriteBatch(b)
}

func (fd *fileDAOLegacy) DeleteTipBlock() error {
	// First obtain tip height from db
	height, err := fd.Height()
	if err != nil {
		return errors.Wrap(err, "failed to get tip height")
	}
	if height == 0 {
		// should not delete genesis block
		return errors.New("cannot delete genesis block")
	}
	// Obtain tip block hash
	hash, err := fd.getTipHash()
	if err != nil {
		return errors.Wrap(err, "failed to get tip block hash")
	}

	b := batch.NewBatch()
	batchForBlock := batch.NewBatch()
	whichDB, _, err := fd.getDBFromHeight(height)
	if err != nil {
		return err
	}
	// Delete hash -> block mapping
	batchForBlock.Delete(blockHeaderNS, hash[:], "failed to delete block header")
	batchForBlock.Delete(blockBodyNS, hash[:], "failed to delete block body")
	batchForBlock.Delete(blockFooterNS, hash[:], "failed to delete block footer")
	// delete receipt
	batchForBlock.Delete(receiptsNS, byteutil.Uint64ToBytes(height), "failed to delete receipt")
	// Delete hash -> height mapping
	hashKey := hashKey(hash)
	b.Delete(blockHashHeightMappingNS, hashKey, "failed to delete hash -> height mapping")
	// Delete height -> hash mapping
	heightKey := heightKey(height)
	b.Delete(blockHashHeightMappingNS, heightKey, "failed to delete height -> hash mapping")

	// Update tip height
	b.Put(blockNS, topHeightKey, byteutil.Uint64ToBytes(height-1), "failed to put top height")
	// Update tip hash
	hash2, err := fd.GetBlockHash(height - 1)
	if err != nil {
		return errors.Wrap(err, "failed to get tip block hash")
	}
	b.Put(blockNS, topHashKey, hash2[:], "failed to put top hash")

	if err := fd.kvStore.WriteBatch(b); err != nil {
		return err
	}
	return whichDB.WriteBatch(batchForBlock)
}

// getTipHash returns the blockchain tip hash
func (fd *fileDAOLegacy) getTipHash() (hash.Hash256, error) {
	value, err := fd.kvStore.Get(blockNS, topHashKey)
	if err != nil {
		return hash.ZeroHash256, errors.Wrap(err, "failed to get tip hash")
	}
	return hash.BytesToHash256(value), nil
}

func (fd *fileDAOLegacy) indexFile(height uint64, index []byte) error {
	fd.mutex.Lock()
	defer fd.mutex.Unlock()

	if fd.htf == nil {
		htf, err := db.NewRangeIndex(fd.kvStore, heightToFileBucket, make([]byte, 8))
		if err != nil {
			return err
		}
		fd.htf = htf
	}
	return fd.htf.Insert(height, index)
}

// getFileIndex return the db filename
func (fd *fileDAOLegacy) getFileIndex(height uint64) ([]byte, error) {
	fd.mutex.RLock()
	defer fd.mutex.RUnlock()

	if fd.htf == nil {
		htf, err := db.NewRangeIndex(fd.kvStore, heightToFileBucket, make([]byte, 8))
		if err != nil {
			return nil, err
		}
		fd.htf = htf
	}
	return fd.htf.Get(height)
}

func (fd *fileDAOLegacy) getTopDB(blkHeight uint64) (kvStore db.KVStore, index uint64, err error) {
	if fd.cfg.SplitDBSizeMB == 0 || blkHeight <= fd.cfg.SplitDBHeight {
		return fd.kvStore, 0, nil
	}
	topIndex := fd.topIndex.Load().(uint64)
	file, dir := getFileNameAndDir(fd.cfg.DbPath)
	if err != nil {
		return
	}
	longFileName := dir + "/" + file + fmt.Sprintf("-%08d", topIndex) + ".db"
	dat, err := os.Stat(longFileName)
	if err != nil {
		if !os.IsNotExist(err) {
			// something wrong getting FileInfo
			return
		}
		// index the height --> file index mapping
		if err = fd.indexFile(blkHeight, byteutil.Uint64ToBytesBigEndian(topIndex)); err != nil {
			return
		}
		// db file does not exist, create it
		return fd.openDB(topIndex)
	}
	// other errors except file does not exist
	if err != nil {
		return
	}
	// file exists,but need create new db
	if uint64(dat.Size()) > fd.cfg.SplitDBSize() {
		kvStore, index, err = fd.openDB(topIndex + 1)
		fd.topIndex.Store(index)
		// index the height --> file index mapping
		err = fd.indexFile(blkHeight, byteutil.Uint64ToBytesBigEndian(index))
		return
	}
	// db exist,need load from kvStores
	kv, ok := fd.kvStores.Load(topIndex)
	if ok {
		kvStore, ok = kv.(db.KVStore)
		if !ok {
			err = errors.New("db convert error")
		}
		index = topIndex
		return
	}
	// file exists,but not opened
	return fd.openDB(topIndex)
}

// getDBFromHash returns db of this block stored
func (fd *fileDAOLegacy) getDBFromHash(h hash.Hash256) (db.KVStore, uint64, error) {
	height, err := fd.GetBlockHeight(h)
	if err != nil {
		return nil, 0, err
	}
	return fd.getDBFromHeight(height)
}

func (fd *fileDAOLegacy) getDBFromHeight(blkHeight uint64) (kvStore db.KVStore, index uint64, err error) {
	if fd.cfg.SplitDBSizeMB == 0 {
		return fd.kvStore, 0, nil
	}
	if blkHeight <= fd.cfg.SplitDBHeight {
		return fd.kvStore, 0, nil
	}
	// get file index
	value, err := fd.getFileIndex(blkHeight)
	if err != nil {
		return
	}
	return fd.getDBFromIndex(byteutil.BytesToUint64BigEndian(value))
}

func (fd *fileDAOLegacy) getDBFromIndex(idx uint64) (kvStore db.KVStore, index uint64, err error) {
	if idx == 0 {
		return fd.kvStore, 0, nil
	}
	kv, ok := fd.kvStores.Load(idx)
	if ok {
		kvStore, ok = kv.(db.KVStore)
		if !ok {
			err = errors.New("db convert error")
		}
		index = idx
		return
	}
	// if user rm some db files manully,then call this method will create new file
	return fd.openDB(idx)
}

// getBlockValue get block's data from db,if this db failed,it will try the previous one
func (fd *fileDAOLegacy) getBlockValue(blockNS string, h hash.Hash256) ([]byte, error) {
	whichDB, index, err := fd.getDBFromHash(h)
	if err != nil {
		return nil, err
	}
	value, err := whichDB.Get(blockNS, h[:])
	if errors.Cause(err) == db.ErrNotExist {
		idx := index - 1
		if index == 0 {
			idx = 0
		}
		db, _, err := fd.getDBFromIndex(idx)
		if err != nil {
			return nil, err
		}
		value, err = db.Get(blockNS, h[:])
		if err != nil {
			return nil, err
		}
	}
	return value, err
}

// openDB open file if exists, or create new file
func (fd *fileDAOLegacy) openDB(idx uint64) (kvStore db.KVStore, index uint64, err error) {
	if idx == 0 {
		return fd.kvStore, 0, nil
	}
	cfg := fd.cfg
	model, _ := getFileNameAndDir(cfg.DbPath)
	name := model + fmt.Sprintf("-%08d", idx) + ".db"

	fd.mutex.Lock()
	defer fd.mutex.Unlock()
	// open or create this db file
	cfg.DbPath = path.Dir(cfg.DbPath) + "/" + name
	var newFile bool
	_, err = os.Stat(cfg.DbPath)
	if err != nil {
		if !os.IsNotExist(err) {
			// something wrong getting FileInfo
			return
		}
		newFile = true
	}

	kvStore = db.NewBoltDB(cfg)
	fd.kvStores.Store(idx, kvStore)
	err = kvStore.Start(context.Background())
	if err != nil {
		return
	}

	if newFile {
		if err = kvStore.Put(systemLogNS, make([]byte, 8), []byte(systemLogNS)); err != nil {
			return
		}
	}
	fd.lifecycle.Add(kvStore)
	index = idx
	return
}

func getFileNameAndDir(p string) (fileName, dir string) {
	var withSuffix, suffix string
	withSuffix = path.Base(p)
	suffix = path.Ext(withSuffix)
	fileName = strings.TrimSuffix(withSuffix, suffix)
	dir = path.Dir(p)
	return
}

func heightKey(height uint64) []byte {
	return append(heightPrefix, byteutil.Uint64ToBytes(height)...)
}
