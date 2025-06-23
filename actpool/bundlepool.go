package actpool

import (
	"context"
	"encoding/hex"
	"sync"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

var (
	ErrNoBundlesForHeight = errors.New("no bundles for the height")
	ErrNilBundle          = errors.New("bundle is nil")
	ErrBundleTargetHeight = errors.New("bundle target block height is invalid")
	ErrBundleExists       = errors.New("bundle already exists")
	ErrBundleUUIDExists   = errors.New("bundle with UUID already exists")
	ErrEmptyBundle        = errors.New("bundle is empty")
	ErrBundleNotFound     = errors.New("bundle not found")
	ErrNilBlock           = errors.New("block is nil")
	ErrInvalidCaller      = errors.New("invalid caller")
	ErrBlockHeightTooLow  = errors.New("block height is too low")
)

type (
	// BroadcastBundleFunc is the interface for broadcasting bundles.
	BroadcastBundleFunc func(ctx context.Context, sender address.Address, uuid string, bundle *action.Bundle)
	meta                struct {
		bundle *action.Bundle
		sender address.Address
		uuid   string
	}

	// BundlePool defines a pool of bundles.
	BundlePool struct {
		mu                    sync.RWMutex
		height                uint64
		metas                 map[hash.Hash256]*meta
		uuids                 map[string]hash.Hash256   // UUID to bundle hash mapping
		targetHeightToBundles map[uint64][]hash.Hash256 // Target block height to bundles mapping
		broadcast             BroadcastBundleFunc       // Function to broadcast bundles
	}
)

// NewBundlePool creates a new BundlePool
func NewBundlePool() *BundlePool {
	return &BundlePool{
		mu:                    sync.RWMutex{},
		height:                0,
		metas:                 make(map[hash.Hash256]*meta),
		uuids:                 make(map[string]hash.Hash256),
		targetHeightToBundles: make(map[uint64][]hash.Hash256),
		broadcast:             nil,
	}
}

func (bp *BundlePool) SetBroadcastHandler(broadcast BroadcastBundleFunc) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.broadcast = broadcast
}

// AddBundle adds a bundle to the pool.
func (bp *BundlePool) AddBundle(ctx context.Context, sender address.Address, uuid string, bundle *action.Bundle) error {
	if bundle == nil {
		return ErrNilBundle
	}
	if bundle.Len() == 0 {
		return ErrEmptyBundle
	}
	h := bundle.Hash()
	height := bundle.TargetBlockHeight()
	bp.mu.Lock()
	defer bp.mu.Unlock()
	if height <= bp.height {
		return errors.Wrapf(ErrBundleTargetHeight, "bundle target block height %d is less than current height %d", height, bp.height)
	}
	if _, ok := bp.metas[h]; ok {
		return errors.Wrapf(ErrBundleExists, "bundle with hash %x already exists", h)
	}
	if _, ok := bp.uuids[uuid]; ok {
		return errors.Wrapf(ErrBundleUUIDExists, "bundle with UUID %s already exists", uuid)
	}
	bp.metas[h] = &meta{
		bundle: bundle,
		uuid:   uuid,
		sender: sender,
	}
	bp.uuids[uuid] = h
	bp.targetHeightToBundles[height] = append(bp.targetHeightToBundles[height], h)
	log.L().Info("Added bundle to pool", zap.String("hash", hex.EncodeToString(h[:])), zap.Uint64("targetHeight", height), zap.String("uuid", uuid))
	if bp.broadcast != nil {
		bp.broadcast(ctx, sender, uuid, bundle)
	}

	return nil
}

// BundlesAtHeight retrieves all bundles at a specific height.
func (bp *BundlePool) BundlesAtHeight(height uint64) ([]string, []address.Address, []*action.Bundle, error) {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	hashes, ok := bp.targetHeightToBundles[height]
	if !ok {
		return nil, nil, nil, errors.Wrapf(ErrNoBundlesForHeight, "no bundles found for height %d", height)
	}
	uuids := make([]string, 0, len(hashes))
	senders := make([]address.Address, 0, len(hashes))
	bundles := make([]*action.Bundle, 0, len(hashes))
	for _, h := range hashes {
		if m, ok := bp.metas[h]; ok {
			bundles = append(bundles, m.bundle)
			uuids = append(uuids, m.uuid)
			senders = append(senders, m.sender)
		}
	}
	return uuids, senders, bundles, nil
}

// Size retrieves the number of bundles in the pool.
func (bp *BundlePool) Size() int {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	return len(bp.metas)
}

// DeleteBundle deletes a bundle from the pool by its UUID.
func (bp *BundlePool) DeleteBundle(caller address.Address, uuid string) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	h, ok := bp.uuids[uuid]
	if !ok {
		return errors.Wrapf(ErrBundleNotFound, "bundle with UUID %s not found", uuid)
	}
	m := bp.metas[h]
	if m.sender != caller {
		return errors.Wrapf(ErrInvalidCaller, "caller %s is not the sender of the bundle with hash %x", caller.String(), h)
	}
	height := m.bundle.TargetBlockHeight()
	// Remove from targetHeightToBundles
	hashes := bp.targetHeightToBundles[height]
	for i, hash := range hashes {
		if hash == h {
			bp.targetHeightToBundles[height] = append(hashes[:i], hashes[i+1:]...)
			break
		}
	}
	delete(bp.uuids, uuid)
	delete(bp.metas, h)

	return nil
}

// Reset clears the bundle pool.
func (bp *BundlePool) Reset() {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.metas = make(map[hash.Hash256]*meta)
	bp.uuids = make(map[string]hash.Hash256)
	bp.targetHeightToBundles = make(map[uint64][]hash.Hash256)
	bp.height = 0
}

// ReceiveBlock handles the reception of a block and removes all bundles at the block's height.
func (bp *BundlePool) ReceiveBlock(blk *block.Block) error {
	if blk == nil {
		return ErrNilBlock
	}
	height := blk.Height()
	bp.mu.Lock()
	defer bp.mu.Unlock()
	if height <= bp.height {
		return errors.Wrapf(ErrBlockHeightTooLow, "block height %d is less than or equal to current height %d", height, bp.height)
	}
	if bp.height == 0 {
		toDelete := make([]uint64, 0, len(bp.targetHeightToBundles))
		for h := range bp.targetHeightToBundles {
			if h <= height {
				toDelete = append(toDelete, h)
			}
		}
		for _, h := range toDelete {
			hashes := bp.targetHeightToBundles[h]
			for _, bundleHash := range hashes {
				uuid := bp.metas[bundleHash].uuid
				delete(bp.metas, bundleHash)
				delete(bp.uuids, uuid)
			}
			delete(bp.targetHeightToBundles, h)
		}
	}
	bp.height = height
	// Remove all bundles at this height
	hashes, ok := bp.targetHeightToBundles[height]
	if ok {
		for _, h := range hashes {
			uuid := bp.metas[h].uuid
			delete(bp.metas, h)
			delete(bp.uuids, uuid)
		}
		delete(bp.targetHeightToBundles, height)
	}
	return nil
}
