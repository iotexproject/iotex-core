// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockdb

import (
	"io/ioutil"
	"os"

	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	cm "github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/config"
	cp "github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/proto"
)

// Directory of block data
const (
	BlockData = "../block.dat"
)

var (
	tipHash    = []byte("tip.hash")
	tipHeight  = []byte("tip.height")
	utxoHeight = []byte("utxo.height")

	// bucket to store serialized block
	blocksBucket = []byte("blocks")

	// bucket to store block height <-> hash
	hashHeightBucket = []byte("hash<->height")
)

var (
	// ErrNotExist indicates certain item does not exist in Blockchain database
	ErrNotExist = errors.New("not exist in DB")
	// ErrAlreadyExist indicates certain item already exists in Blockchain database
	ErrAlreadyExist = errors.New("already exist in DB")
)

// BlockDB defines the DB interface to read/store/persist blocks
type BlockDB struct {
	*bolt.DB
}

// NewBlockDB returns a new BlockDB instance
func NewBlockDB(cfg *config.Config) (*BlockDB, bool) {
	exist := fileExists(cfg.Chain.ChainDBPath)

	// create/open database file
	db, err := bolt.Open(cfg.Chain.ChainDBPath, 0600, nil)
	if err != nil {
		glog.Fatalf("Failed to open Blockchain Db, error = %v", err)
		return nil, exist
	}

	if !exist {
		// create buckets
		if err := db.Update(func(tx *bolt.Tx) error {
			b, err := tx.CreateBucket(blocksBucket)
			if err != nil {
				return errors.Wrap(err, "Creating bucket for blocks")
			}

			// set init value for tip hash and height
			if err := b.Put(tipHash, cp.ZeroHash32B[:]); err != nil {
				return errors.Wrap(err, "Writing init value for tipHash")
			}

			zero := []byte{0, 0, 0, 0}
			if err := b.Put(tipHeight, zero); err != nil {
				return errors.Wrap(err, "Writing init value for tipHeight")
			}

			if b, err = tx.CreateBucket(hashHeightBucket); err != nil {
				return errors.Wrap(err, "Creating bucket for hash <-> height mapping")
			}
			return nil
		}); err != nil {
			glog.Fatal(err)
			return nil, exist
		}
	}
	return &BlockDB{db}, exist
}

// Init initializes the BlockDB instance
func (db *BlockDB) Init() (hash []byte, height uint32, err error) {
	// verify all buckets are properly created
	// so from this point on later calls don't need to sanity check again
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(hashHeightBucket)
		if b == nil {
			return errors.Wrap(bolt.ErrBucketNotFound, "Bucket for hash <-> height mapping")
		}

		b = tx.Bucket(blocksBucket)
		if b == nil {
			return errors.Wrap(bolt.ErrBucketNotFound, "Bucket for blocks")
		}

		// get tip hash and height
		if hash = b.Get(tipHash); hash == nil {
			return errors.Wrap(ErrNotExist, "Blockchain tip")
		}

		h := b.Get(tipHeight)
		if h == nil {
			return errors.Wrap(ErrNotExist, "Blockchain height")
		}
		height = cm.MachineEndian.Uint32(h)
		return nil
	})
	return
}

// GetBlockHash returns the block hash by height
func (db *BlockDB) GetBlockHash(height uint32) (hash []byte, err error) {
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(hashHeightBucket)
		// get block hash at passed-in height
		dbHeight := []byte{0, 0, 0, 0}
		cm.MachineEndian.PutUint32(dbHeight, height)
		if hash = b.Get(dbHeight); hash == nil {
			return errors.Wrapf(ErrNotExist, "Block with height = %d", height)
		}
		return nil
	})
	return
}

// GetBlockHeight returns the block height by hash
func (db *BlockDB) GetBlockHeight(hash []byte) (height uint32, err error) {
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(hashHeightBucket)
		// get the height corresponding to passed in hash
		var dbHeight []byte
		if dbHeight = b.Get(hash); dbHeight == nil {
			return errors.Wrapf(ErrNotExist, "Block with hash = %x", hash)
		}
		height = cm.MachineEndian.Uint32(dbHeight)
		return nil
	})
	return
}

// CheckOutBlock checks a block out of DB
func (db *BlockDB) CheckOutBlock(hash []byte) (blk []byte, err error) {
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(blocksBucket)

		// get the block with passed-in hash
		if blk = b.Get(hash); blk == nil {
			return errors.Wrapf(ErrNotExist, "Block with hash = %x", hash)
		}
		return nil
	})
	return
}

// CheckInBlock checks a block into DB
func (db *BlockDB) CheckInBlock(blk []byte, hash []byte, h uint32) error {
	return db.Update(func(tx *bolt.Tx) error {
		// new block hash should not collide with any existing blocks
		b := tx.Bucket(blocksBucket)
		if collide := b.Get(hash); collide != nil {
			return errors.Wrapf(ErrAlreadyExist, "New block hash %x", hash)
		}

		// prepare tip height
		height := []byte{0, 0, 0, 0}
		cm.MachineEndian.PutUint32(height, h)

		// update tip hash/height
		if err := b.Put(tipHash, hash); err != nil {
			return errors.Wrapf(err, "Writing tipHash = %x", hash)
		}

		if err := b.Put(tipHeight, height); err != nil {
			return errors.Wrapf(err, "Writing tipHeight = %v", height)
		}

		// commit the block data into Db
		if err := b.Put(hash, blk); err != nil {
			return errors.Wrapf(err, "Writing block = %x", hash)
		}

		// update hash <-> height mapping
		b = tx.Bucket(hashHeightBucket)
		if err := b.Put(hash, height); err != nil {
			return errors.Wrapf(err, "Updating hash <-> height mapping height = %v", height)
		}

		if err := b.Put(height, hash); err != nil {
			return errors.Wrapf(err, "Updating hash <-> height mapping hash = %x", hash)
		}
		return nil
	})
}

// StoreBlockToFile writes block raw data into file
func (db *BlockDB) StoreBlockToFile(start, end uint32) error {
	data := []byte{}
	offset := []uint32{}
	seek := uint32(0)
	for height := start; height <= end; height++ {
		hash, err := db.GetBlockHash(height)
		if err != nil {
			return err
		}
		bytes, err := db.CheckOutBlock(hash[:])
		if err != nil {
			return err
		}
		offset = append(offset, seek)
		seek += uint32(len(bytes))
		data = append(data, bytes...)
	}
	offset = append(offset, seek)

	// create block index
	index, err := proto.Marshal(&iproto.BlockIndex{start, end, offset})
	if err != nil {
		return err
	}

	// first 4-byte of file is the size of block index
	size := []byte{0, 0, 0, 0}
	cm.MachineEndian.PutUint32(size, uint32(len(index)))
	file := append(size, index...)
	file = append(file, data...)
	return ioutil.WriteFile(BlockData, file, 0600)
}

// fileExists checks if a file already exists
func fileExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}
