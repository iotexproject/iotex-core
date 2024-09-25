// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/go-pkgs/byteutil"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/routine"
)

var (
	_writeHeight   = []byte("wh")
	_expireHeight  = []byte("eh")
	_blobDataNS    = "blb"
	_heightIndexNS = "hin" // mapping from blob height to index
	_hashHeightNS  = "shn" // mapping from action hash to blob height
)

type (
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
		kvStore                         db.KVStore
		totalBlocks                     uint64
		currWriteBlock, currExpireBlock uint64
		purgeTask                       *routine.RecurringTask
		interval                        time.Duration
	}
)

func NewBlobStore(kv db.KVStore, size uint64, interval time.Duration) *blobStore {
	return &blobStore{
		kvStore:     kv,
		totalBlocks: size,
		interval:    interval,
	}
}

func (bs *blobStore) Start(ctx context.Context) error {
	if err := bs.kvStore.Start(ctx); err != nil {
		return err
	}
	if err := bs.checkDB(); err != nil {
		return err
	}
	if bs.interval != 0 {
		bs.purgeTask = routine.NewRecurringTask(func() {
			bs.expireBlob(atomic.LoadUint64(&bs.currWriteBlock))
		}, bs.interval)
		return bs.purgeTask.Start(ctx)
	}
	return nil
}

func (bs *blobStore) Stop(ctx context.Context) error {
	if err := bs.purgeTask.Stop(ctx); err != nil {
		return err
	}
	return bs.kvStore.Stop(ctx)
}

func (bs *blobStore) checkDB() error {
	var err error
	bs.currWriteBlock, err = bs.getHeightByHash(_writeHeight)
	if err != nil && errors.Cause(err) != db.ErrNotExist {
		return err
	}
	bs.currExpireBlock, err = bs.getHeightByHash(_expireHeight)
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
	if blk.Height() <= atomic.LoadUint64(&bs.currWriteBlock) {
		return errors.Errorf("block height %d is less than current tip height", blk.Height())
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
	raw, err := proto.Marshal(&pb)
	if err != nil {
		return errors.Wrapf(err, "failed to put block = %d", blk.Height())
	}
	return bs.putBlob(raw, blk.Height(), pb.TxHash)
}

func (bs *blobStore) putBlob(blob []byte, height uint64, txHash [][]byte) error {
	// write blob index
	var (
		b     = batch.NewBatch()
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
	if err := bs.kvStore.WriteBatch(b); err != nil {
		return err
	}
	atomic.StoreUint64(&bs.currWriteBlock, height)
	return nil
}

func (bs *blobStore) expireBlob(height uint64) error {
	var (
		b              = batch.NewBatch()
		toExpireHeight = height - bs.totalBlocks
	)
	if height <= bs.totalBlocks || toExpireHeight <= bs.currExpireBlock {
		return nil
	}
	ek, ev, err := bs.kvStore.Filter(_heightIndexNS, func(k, v []byte) bool { return true },
		keyForBlock(bs.currExpireBlock), keyForBlock(toExpireHeight))
	if errors.Cause(err) == db.ErrNotExist {
		// no blobs are stored up to the new expire height
		bs.currExpireBlock = toExpireHeight
		return nil
	}
	if err != nil {
		return err
	}
	for _, v := range ev {
		dx, err := deserializeBlobIndex(v)
		if err != nil {
			return err
		}
		// delete tx hash of expired blobs
		for i := range dx.hashes {
			b.Delete(_hashHeightNS, dx.hashes[i], "failed to delete hash")
		}
	}
	for i := range ek {
		// delete index expired blob data
		b.Delete(_heightIndexNS, ek[i], "failed to delete index")
		b.Delete(_blobDataNS, ek[i], "failed to delete blob")
	}
	// update expired blob height
	b.Put(_hashHeightNS, _expireHeight, keyForBlock(toExpireHeight), "failed to put expired height")
	if err := bs.kvStore.WriteBatch(b); err != nil {
		return err
	}
	bs.currExpireBlock = toExpireHeight
	return nil
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
