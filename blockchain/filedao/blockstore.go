package filedao

import (
	"fmt"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const (
	blockstoreDefaultVersion = byte(0)
	blockstoreHeaderSize     = 8
)

type (
	// blockstore is a byte slice that contains a block, its receipts and transaction logs
	// the first 8 bytes is the size of the block
	// the next n bytes is the serialized block and receipts, n is the size of the block
	// the rest bytes is the serialized transaction logs
	blockstore []byte
)

var (
	errInvalidBlockstore = fmt.Errorf("invalid blockstore")
)

func convertToBlockStore(blk *block.Block) (blockstore, error) {
	data := make(blockstore, 0)
	s := &block.Store{
		Block:    blk,
		Receipts: blk.Receipts,
	}
	tmp, err := s.Serialize()
	if err != nil {
		return nil, err
	}
	data = append(data, blockstoreDefaultVersion)
	data = append(data, byteutil.Uint64ToBytesBigEndian(uint64(len(tmp)))...)
	data = append(data, tmp...)
	txLog := blk.TransactionLog()
	if txLog != nil {
		tmp = txLog.Serialize()
		data = append(data, tmp...)
	}
	return data, nil
}

func (s blockstore) Block(deser *block.Deserializer) (*block.Block, error) {
	size := s.blockSize()
	bs, err := deser.DeserializeBlockStore(s[blockstoreHeaderSize : size+blockstoreHeaderSize])
	if err != nil {
		return nil, err
	}
	bs.Block.Receipts = bs.Receipts
	return bs.Block, nil
}

func (s blockstore) TransactionLogs() (*iotextypes.TransactionLogs, error) {
	size := s.blockSize()
	if uint64(len(s)) == size+blockstoreHeaderSize {
		return nil, nil
	}
	return block.DeserializeSystemLogPb(s[size+blockstoreHeaderSize:])
}

func (s blockstore) Serialize() []byte {
	return append([]byte{blockstoreDefaultVersion}, s...)
}

func (s *blockstore) Deserialize(data []byte) error {
	if len(data) == 0 {
		return errInvalidBlockstore
	}
	switch data[0] {
	case blockstoreDefaultVersion:
		bs := blockstore(data[1:])
		if err := bs.Validate(); err != nil {
			return err
		}
		*s = bs
		return nil
	default:
		return errInvalidBlockstore
	}
}

func (s blockstore) blockSize() uint64 {
	return byteutil.BytesToUint64BigEndian(s[:blockstoreHeaderSize])
}

func (s blockstore) Validate() error {
	if len(s) < blockstoreHeaderSize {
		return errInvalidBlockstore
	}
	blkSize := s.blockSize()
	if uint64(len(s)) < blkSize+blockstoreHeaderSize {
		return errInvalidBlockstore
	}
	return nil
}
