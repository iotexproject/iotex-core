package filedao

import (
	"context"
	"os"
	"sync"

	"github.com/holiman/billy"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/log"
)

type (
	sizedDao struct {
		size    uint64
		dataDir string

		store billy.Database

		tip          uint64
		base         uint64
		heightToHash map[uint64]hash.Hash256
		hashToHeight map[hash.Hash256]uint64
		heightToID   map[uint64]uint64
		lock         sync.RWMutex

		deser *block.Deserializer
	}
)

func NewSizedFileDao(size uint64, dataDir string, deser *block.Deserializer) (FileDAO, error) {
	return &sizedDao{
		size:         size,
		dataDir:      dataDir,
		heightToHash: make(map[uint64]hash.Hash256),
		hashToHeight: make(map[hash.Hash256]uint64),
		heightToID:   make(map[uint64]uint64),
		deser:        deser,
	}, nil
}

func (sd *sizedDao) Start(ctx context.Context) error {
	sd.lock.Lock()
	defer sd.lock.Unlock()
	dir := sd.dataDir
	if err := os.MkdirAll(dir, 0700); err != nil {
		return errors.Wrap(err, "failed to create blob store directory")
	}

	var fails []uint64
	index := func(id uint64, size uint32, blob []byte) {
		bs := new(blockstore)
		err := bs.Deserialize(blob)
		if err != nil {
			fails = append(fails, id)
			log.L().Warn("Failed to decode block store", zap.Error(err))
			return
		}
		blk, err := bs.Block(sd.deser)
		if err != nil {
			fails = append(fails, id)
			log.L().Warn("Failed to decode block", zap.Error(err))
			return
		}
		h := blk.HashBlock()
		height := blk.Height()
		sd.hashToHeight[h] = height
		sd.heightToHash[height] = h
		sd.heightToID[height] = id
		if height > sd.tip || sd.tip == 0 {
			sd.tip = height
		}
		if height < sd.base || sd.base == 0 {
			sd.base = height
		}
	}

	store, err := billy.Open(billy.Options{Path: dir}, newSlotter(), index)
	if err != nil {
		return errors.Wrap(err, "failed to open blob store")
	}
	sd.store = store
	if len(fails) > 0 {
		return errors.Errorf("failed to decode blocks %v", fails)
	}
	return nil
}

func (sd *sizedDao) Stop(ctx context.Context) error {
	sd.lock.Lock()
	defer sd.lock.Unlock()
	return sd.store.Close()
}

func (sd *sizedDao) PutBlock(ctx context.Context, blk *block.Block) error {
	if blk.Height() != sd.tip+1 {
		return ErrInvalidTipHeight
	}
	bs, err := convertToBlockStore(blk)
	if err != nil {
		return err
	}

	sd.lock.Lock()
	defer sd.lock.Unlock()
	if blk.Height() != sd.tip+1 {
		return ErrInvalidTipHeight
	}
	id, err := sd.store.Put(bs.Serialize())
	if err != nil {
		return err
	}
	sd.tip++
	hash := blk.HashBlock()
	sd.heightToHash[sd.tip] = hash
	sd.hashToHeight[hash] = sd.tip
	sd.heightToID[sd.tip] = id

	if sd.tip-sd.base > sd.size {
		sd.drop()
	}
	return nil
}

func (sd *sizedDao) Height() (uint64, error) {
	sd.lock.RLock()
	defer sd.lock.RUnlock()
	return sd.tip, nil
}

func (sd *sizedDao) GetBlockHash(height uint64) (hash.Hash256, error) {
	sd.lock.RLock()
	defer sd.lock.RUnlock()
	h, ok := sd.heightToHash[height]
	if !ok {
		return hash.ZeroHash256, db.ErrNotExist
	}
	return h, nil
}

func (sd *sizedDao) GetBlockHeight(h hash.Hash256) (uint64, error) {
	sd.lock.RLock()
	defer sd.lock.RUnlock()
	height, ok := sd.hashToHeight[h]
	if !ok {
		return 0, db.ErrNotExist
	}
	return height, nil
}

func (sd *sizedDao) GetBlock(h hash.Hash256) (*block.Block, error) {
	sd.lock.RLock()
	defer sd.lock.RUnlock()
	height, ok := sd.hashToHeight[h]
	if !ok {
		return nil, db.ErrNotExist
	}
	return sd.getBlock(height)
}

func (sd *sizedDao) GetBlockByHeight(height uint64) (*block.Block, error) {
	sd.lock.RLock()
	defer sd.lock.RUnlock()
	return sd.getBlock(height)
}

func (sd *sizedDao) GetReceipts(height uint64) ([]*action.Receipt, error) {
	sd.lock.RLock()
	defer sd.lock.RUnlock()
	blk, err := sd.getBlock(height)
	if err != nil {
		return nil, err
	}
	return blk.Receipts, nil
}

func (sd *sizedDao) ContainsTransactionLog() bool {
	return true
}

func (sd *sizedDao) TransactionLogs(height uint64) (*iotextypes.TransactionLogs, error) {
	sd.lock.RLock()
	defer sd.lock.RUnlock()
	id, ok := sd.heightToID[height]
	if !ok {
		return nil, db.ErrNotExist
	}
	data, err := sd.store.Get(id)
	if err != nil {
		return nil, err
	}
	bs := new(blockstore)
	err = bs.Deserialize(data)
	if err != nil {
		return nil, err
	}
	return bs.TransactionLogs()
}

func (sd *sizedDao) DeleteTipBlock() error {
	panic("not supported")
}

func (sd *sizedDao) Header(h hash.Hash256) (*block.Header, error) {
	sd.lock.RLock()
	defer sd.lock.RUnlock()
	height, ok := sd.hashToHeight[h]
	if !ok {
		return nil, db.ErrNotExist
	}
	blk, err := sd.getBlock(height)
	if err != nil {
		return nil, err
	}
	return &blk.Header, nil
}

func (sd *sizedDao) HeaderByHeight(height uint64) (*block.Header, error) {
	sd.lock.RLock()
	defer sd.lock.RUnlock()
	blk, err := sd.getBlock(height)
	if err != nil {
		return nil, err
	}
	return &blk.Header, nil
}

func (sd *sizedDao) FooterByHeight(height uint64) (*block.Footer, error) {
	sd.lock.RLock()
	defer sd.lock.RUnlock()
	blk, err := sd.getBlock(height)
	if err != nil {
		return nil, err
	}
	return &blk.Footer, nil
}

func (sd *sizedDao) getBlock(height uint64) (*block.Block, error) {
	id, ok := sd.heightToID[height]
	if !ok {
		return nil, db.ErrNotExist
	}
	data, err := sd.store.Get(id)
	if err != nil {
		return nil, err
	}
	bs := new(blockstore)
	err = bs.Deserialize(data)
	if err != nil {
		return nil, err
	}
	return bs.Block(sd.deser)
}

func (sd *sizedDao) drop() {
	id := sd.heightToID[sd.base]
	if err := sd.store.Delete(id); err != nil {
		log.L().Error("Failed to delete block", zap.Error(err))
		return
	}
	hash := sd.heightToHash[sd.base]
	delete(sd.heightToHash, sd.base)
	delete(sd.heightToID, sd.base)
	delete(sd.hashToHeight, hash)
	sd.base++
}

func newSlotter() func() (uint32, bool) {
	sizeList := []uint32{
		1024 * 4, // empty block
		1024 * 8, // 2 execution
		1024 * 16,
		1024 * 128, // 250 transfer
		1024 * 512,
		1024 * 1024,
		1024 * 1024 * 4, // 5000 transfer
		1024 * 1024 * 8,
		1024 * 1024 * 16,
		1024 * 1024 * 128,
		1024 * 1024 * 512,
		1024 * 1024 * 1024, // max block size
	}
	i := -1
	return func() (size uint32, done bool) {
		i++
		if i >= len(sizeList)-1 {
			return sizeList[i], true
		}
		return sizeList[i], true
	}
}
