// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/go-pkgs/byteutil"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/batch"
)

var (
	_writeHeight   = []byte("wh")
	_expireHeight  = []byte("eh")
	_blobDataNS    = "blb"
	_heightIndexNS = "hin" // mapping from blob height to index
	_hashHeightNS  = "shn" // mapping from action hash to blob height
)

type (
	// BlobStore defines the interface of blob store
	BlobStore interface {
		Start(context.Context) error
		Stop(context.Context) error
		GetBlob(hash.Hash256) (*types.BlobTxSidecar, string, error)
		GetBlobsByHeight(uint64) ([]*types.BlobTxSidecar, []string, error)
		PutBlock(*block.Block) error
	}

	// storage for past N-day's blobs, structured as blow:
	// 1. Block height is used as key to store blobs. When a new blob is stored,
	//    the expired time (current - 18 days) is updated and all blobs earlier
	//    than this time will be deleted.
	// 2. Two mappings are kept in a separate index: (1) height to tx hashes in,
	//    the blob, and (2) tx hash to the blob's height. Here's an example:
	//    304 --> { []hash: eddf9e61fb9d8f5111840daef55e5fde
	//                      95977fb340867fa13f787df864ed5388 }
	//    and:
	//    eddf9e61fb9d8f5111840daef55e5fde --> {height: 311344}
	//    95977fb340867fa13f787df864ed5388 --> {height: 311344}
	//    The 2nd mapping is used to support GetBlob(hash) function. It converts
	//    the requested blob hash to the height, and retrieves the blob.
	//    When new blobs are added to the storage, the index of expired blobs will
	//    be deleted correspondingly.
	// 3. The maximum number of blobs is 6 for each block, so maximum data size
	//    stored by a key is 131kB x 6 = 786kB. The maximum total size of the
	//    entire blob storage is 786kB x 311040 = 245GB.
	//
	blobStore struct {
		kvStore        db.KVStore
		totalBlocks    uint64
		currWriteBlock uint64
	}
)

func NewBlobStore(kv db.KVStore, size uint64) *blobStore {
	return &blobStore{
		kvStore:     kv,
		totalBlocks: size,
	}
}

func (bs *blobStore) Start(ctx context.Context) error {
	if err := bs.kvStore.Start(ctx); err != nil {
		return err
	}
	return bs.checkDB()
}

func (bs *blobStore) Stop(ctx context.Context) error {
	return bs.kvStore.Stop(ctx)
}

func (bs *blobStore) checkDB() error {
	var err error
	bs.currWriteBlock, err = bs.getHeightByHash(_writeHeight)
	if err != nil && errors.Cause(err) != db.ErrNotExist {
		return err
	}
	// in case the retention window size has shrunk, do a one-time purge
	return bs.expireBlob(bs.currWriteBlock)
}

func (bs *blobStore) GetBlob(h hash.Hash256) (*types.BlobTxSidecar, string, error) {
	height, err := bs.getHeightByHash(h[:])
	if err != nil {
		return nil, "", err
	}
	blobs, hashes, err := bs.getBlobs(height)
	if err != nil {
		return nil, "", err
	}
	target := common.BytesToHash(h[:]).Hex()
	for i, v := range hashes {
		if v == target {
			return blobs[i], v, nil
		}
	}
	return nil, "", errors.Errorf("data inconsistency: cannot find blob hash = %s", target)
}

func (bs *blobStore) GetBlobsByHeight(height uint64) ([]*types.BlobTxSidecar, []string, error) {
	return bs.getBlobs(height)
}

func (bs *blobStore) getBlobs(height uint64) ([]*types.BlobTxSidecar, []string, error) {
	raw, err := bs.kvStore.Get(_blobDataNS, keyForBlock(height))
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to get blobs at height %d", height)
	}
	blobs, hashes, err := decodeBlob(raw)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to decode blobs at height %d", height)
	}
	return blobs, hashes, nil
}

func (bs *blobStore) PutBlock(blk *block.Block) error {
	height := blk.Height()
	if height <= atomic.LoadUint64(&bs.currWriteBlock) {
		return errors.Errorf("block height %d is less than current tip height", height)
	}
	pb := iotextypes.BlobTxSidecars{
		TxHash:   make([][]byte, 0),
		Sidecars: make([]*iotextypes.BlobTxSidecar, 0),
	}
	for _, act := range blk.Actions {
		if b := act.BlobTxSidecar(); b != nil {
			h, err := act.Hash()
			if err != nil {
				return err
			}
			pb.TxHash = append(pb.TxHash, h[:])
			pb.Sidecars = append(pb.Sidecars, action.ToProtoSideCar(b))
		}
	}
	var (
		raw []byte
		err error
	)
	if len(pb.Sidecars) != 0 {
		raw, err = proto.Marshal(&pb)
		if err != nil {
			return errors.Wrapf(err, "failed to put block = %d", height)
		}
	}
	b := batch.NewBatch()
	if raw != nil {
		bs.putBlob(raw, height, pb.TxHash, b)
	}
	if height >= bs.totalBlocks {
		k := keyForBlock(height - bs.totalBlocks)
		v, err := bs.kvStore.Get(_heightIndexNS, k)
		if err != nil {
			if errors.Cause(err) != db.ErrNotExist {
				return err
			}
		} else {
			if err := bs.deleteBlob(k, v, b); err != nil {
				return errors.Wrapf(err, "failed to delete blob")
			}
		}
	}
	if err := bs.kvStore.WriteBatch(b); err != nil {
		return errors.Wrapf(err, "failed to write batch")
	}
	atomic.StoreUint64(&bs.currWriteBlock, height)

	return nil
}

func (bs *blobStore) putBlob(blob []byte, height uint64, txHash [][]byte, b batch.KVStoreBatch) {
	// write blob index
	var (
		key   = keyForBlock(height)
		index = blobIndex{hashes: txHash}
	)
	b.Put(_heightIndexNS, key, index.serialize(), "failed to put height to index mapping")
	// update hash to height
	for i := range txHash {
		b.Put(_hashHeightNS, txHash[i], key, "failed to put hash to height mapping")
	}
	// write the blob data and height
	b.Put(_blobDataNS, key, blob, "failed to put blob")
	b.Put(_hashHeightNS, _writeHeight, key, "failed to put write height")
}

func (bs *blobStore) deleteBlob(k []byte, v []byte, b batch.KVStoreBatch) error {
	dx, err := deserializeBlobIndex(v)
	if err != nil {
		return err
	}
	// delete tx hash of expired blobs
	for i := range dx.hashes {
		b.Delete(_hashHeightNS, dx.hashes[i], "failed to delete hash")
	}
	b.Delete(_heightIndexNS, k, "failed to delete index")
	b.Delete(_blobDataNS, k, "failed to delete blob")

	return nil
}

func (bs *blobStore) expireBlob(height uint64) error {
	var (
		b              = batch.NewBatch()
		toExpireHeight = height - bs.totalBlocks
	)
	if height <= bs.totalBlocks {
		return nil
	}
	ek, ev, err := bs.kvStore.Filter(_heightIndexNS, func(k, v []byte) bool {
		return byteutil.BytesToUint64BigEndian(k) <= toExpireHeight
	}, nil, nil)
	if errors.Cause(err) == db.ErrNotExist {
		// no blobs are stored up to the new expire height
		return nil
	}
	if err != nil {
		return err
	}
	for i, key := range ek {
		v := ev[i]
		if err := bs.deleteBlob(key, v, b); err != nil {
			return errors.Wrapf(err, "failed to delete blob")
		}
	}
	return bs.kvStore.WriteBatch(b)
}

func decodeBlob(raw []byte) ([]*types.BlobTxSidecar, []string, error) {
	pb := iotextypes.BlobTxSidecars{}
	if err := proto.Unmarshal(raw, &pb); err != nil {
		return nil, nil, err
	}
	var (
		pbBlobs  = pb.GetSidecars()
		pbHashes = pb.GetTxHash()
		blobs    = make([]*types.BlobTxSidecar, len(pbBlobs))
		hashes   = make([]string, len(pbBlobs))
		err      error
	)
	for i := range pbBlobs {
		blobs[i], err = action.FromProtoBlobTxSideCar(pbBlobs[i])
		if err != nil {
			return nil, nil, err
		}
		hashes[i] = common.BytesToHash(pbHashes[i]).Hex()
	}
	return blobs, hashes, nil
}

func (bs *blobStore) getHeightByHash(h []byte) (uint64, error) {
	height, err := bs.kvStore.Get(_hashHeightNS, h)
	if err != nil {
		return 0, err
	}
	return byteutil.BytesToUint64BigEndian(height), nil
}
