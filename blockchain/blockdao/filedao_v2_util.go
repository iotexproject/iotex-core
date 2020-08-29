package blockdao

import (
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/compress"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

func (fd *fileDAONew) putTipHashHeightMapping(blk *block.Block) error {
	// write height <-> hash mapping
	h := blk.HashBlock()
	if err := addOneEntryToBatch(fd.hashStore, h[:], fd.batch); err != nil {
		return err
	}

	// write hash <-> height mapping
	heightValue := byteutil.Uint64ToBytesBigEndian(blk.Height())
	fd.batch.Put(blockHashHeightMappingNS, hashKey(h), heightValue, "failed to put hash -> height mapping")

	// update tip hash/height
	if blk.Height() > fd.tipHeight {
		fd.batch.Put(blockHashHeightMappingNS, topHeightKey, heightValue, "failed to put tip height")
		fd.batch.Put(blockHashHeightMappingNS, topHashKey, h[:], "failed to put tip hash")
	}
	return nil
}

func (fd *fileDAONew) putBlock(blk *block.Block) error {
	blkInfo := &block.Store{
		Block:    blk,
		Receipts: blk.Receipts,
	}
	ser, err := blkInfo.Serialize()
	if err != nil {
		return err
	}

	blkBytes, err := compressDatabytes(ser, fd.compressBlock)
	if err != nil {
		return err
	}
	return addOneEntryToBatch(fd.blkStore, blkBytes, fd.batch)
}

func (fd *fileDAONew) putTransactionLog(blk *block.Block) error {
	compressData := fd.compressBlock
	sysLog := blk.TransactionLog()
	if sysLog == nil {
		sysLog = &block.BlkTransactionLog{}
		compressData = false
	}
	logBytes, err := compressDatabytes(sysLog.Serialize(), compressData)
	if err != nil {
		return err
	}
	return addOneEntryToBatch(fd.sysStore, logBytes, fd.batch)
}

func addOneEntryToBatch(c db.CountingIndex, v []byte, b batch.KVStoreBatch) error {
	if err := c.UseBatch(b); err != nil {
		return err
	}
	if err := c.Add(v, true); err != nil {
		return err
	}
	return c.Finalize()
}

func compressDatabytes(value []byte, comp bool) ([]byte, error) {
	if value == nil {
		return nil, ErrDataCorruption
	}

	// append 1 byte to end of bytestream to indicate the block is compressed or not
	if !comp {
		return append(value, _normal), nil
	}

	var err error
	value, err = compress.Compress(value)
	if err != nil {
		return nil, err
	}
	return append(value, _compressed), nil
}

func decompressDatabytes(value []byte) ([]byte, error) {
	if len(value) == 0 {
		return nil, ErrDataCorruption
	}

	// last byte of bytestream indicates the block is compressed or not
	switch value[len(value)-1] {
	case _normal:
		return value[:len(value)-1], nil
	case _compressed:
		return compress.Decompress(value[:len(value)-1])
	default:
		return nil, ErrDataCorruption
	}
}

func getValueMustBe8Bytes(kvs db.KVStore, ns string, key []byte) ([]byte, error) {
	value, err := kvs.Get(ns, key)
	if err != nil {
		return nil, err
	}
	if len(value) != 8 {
		return nil, ErrDataCorruption
	}
	return value, nil
}

func blockStoreKey(height, start uint64) uint64 {
	if height <= start {
		return 0
	}
	return height - start
}

func (fd *fileDAONew) getBlockInfo(height uint64) (*block.Store, error) {
	if !fd.ContainsHeight(height) {
		return nil, db.ErrNotExist
	}

	value, err := fd.blkStore.Get(blockStoreKey(height, fd.bottomHeight))
	if err != nil {
		return nil, err
	}
	value, err = decompressDatabytes(value)
	if err != nil {
		return nil, err
	}
	return extractBlockInfoFromBytes(value)
}

func extractBlockInfoFromBytes(buf []byte) (*block.Store, error) {
	blkInfo := &block.Store{}
	if err := blkInfo.Deserialize(buf); err != nil {
		return nil, err
	}
	return blkInfo, nil
}
