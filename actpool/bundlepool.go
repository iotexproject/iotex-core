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
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

const (
	// DefaultMaxBundleLookahead is the default maximum number of blocks ahead a bundle can target.
	DefaultMaxBundleLookahead = uint64(100)
	// DefaultMaxBundlePoolSize is the default maximum number of bundles in the pool.
	DefaultMaxBundlePoolSize = 1000
)

var (
	ErrNoBundlesForHeight = errors.New("no bundles for the height")
	ErrNilBundle          = errors.New("bundle is nil")
	ErrBundleTargetHeight = errors.New("bundle target block height is invalid")
	ErrBundleExists       = errors.New("bundle already exists")
	ErrBundleUUIDExists   = errors.New("bundle with UUID already exists")
	ErrEmptyBundle        = errors.New("bundle is empty")
	ErrBundleNotFound     = errors.New("bundle not found")
	ErrBundlePoolFull     = errors.New("bundle pool is full")
	ErrNilBlock           = errors.New("block is nil")
	ErrBlockHeightTooLow  = errors.New("block height is too low")
)

type (
	// ValidateActionFunc is the function validating an action.
	ValidateActionFunc func(context.Context, *action.SealedEnvelope) error
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
		genesis               genesis.Genesis
		maxLookahead          uint64 // max blocks ahead a bundle can target
		maxSize               int    // max total bundles in the pool
		metas                 map[hash.Hash256]*meta
		uuids                 map[string]hash.Hash256   // UUID to bundle hash mapping
		targetHeightToBundles map[uint64][]hash.Hash256 // Target block height to bundles mapping
		broadcast             BroadcastBundleFunc       // Function to broadcast bundles
		validate              ValidateActionFunc        // Function to validate actions in bundles
	}
)

// NewBundlePool creates a new BundlePool
func NewBundlePool(g genesis.Genesis) *BundlePool {
	return &BundlePool{
		mu:                    sync.RWMutex{},
		height:                0,
		genesis:               g,
		maxLookahead:          DefaultMaxBundleLookahead,
		maxSize:               DefaultMaxBundlePoolSize,
		metas:                 make(map[hash.Hash256]*meta),
		uuids:                 make(map[string]hash.Hash256),
		targetHeightToBundles: make(map[uint64][]hash.Hash256),
		broadcast:             nil,
		validate:              nil,
	}
}

// SetMaxLookahead sets the maximum number of blocks ahead a bundle can target.
func (bp *BundlePool) SetMaxLookahead(n uint64) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.maxLookahead = n
}

// SetMaxSize sets the maximum number of bundles allowed in the pool.
func (bp *BundlePool) SetMaxSize(n int) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.maxSize = n
}

func (bp *BundlePool) SetValidator(validateFunc ValidateActionFunc) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.validate = validateFunc
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
	if height > bp.height+bp.maxLookahead {
		return errors.Wrapf(ErrBundleTargetHeight, "bundle target block height %d exceeds max lookahead of %d blocks (current height %d)", height, bp.maxLookahead, bp.height)
	}
	if len(bp.metas) >= bp.maxSize {
		return errors.Wrapf(ErrBundlePoolFull, "bundle pool has reached its maximum size of %d", bp.maxSize)
	}
	if _, ok := bp.metas[h]; ok {
		return errors.Wrapf(ErrBundleExists, "bundle with hash %x already exists", h)
	}
	if _, ok := bp.uuids[uuid]; ok {
		return errors.Wrapf(ErrBundleUUIDExists, "bundle with UUID %s already exists", uuid)
	}
	if bp.validate != nil {
		blkCtx := protocol.WithFeatureCtx(protocol.WithBlockCtx(
			genesis.WithGenesisContext(ctx, bp.genesis), protocol.BlockCtx{
				BlockHeight: height,
			}))
		if err := bundle.ForEach(func(selp *action.SealedEnvelope) error {
			return bp.validate(blkCtx, selp)
		}); err != nil {
			return errors.Wrapf(err, "failed to validate bundle %s", uuid)
		}
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
// The UUID itself is the ownership token — only the original submitter knows it.
func (bp *BundlePool) DeleteBundle(uuid string) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	h, ok := bp.uuids[uuid]
	if !ok {
		return errors.Wrapf(ErrBundleNotFound, "bundle with UUID %s not found", uuid)
	}
	m := bp.metas[h]
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
	if len(bp.targetHeightToBundles[height]) == 0 {
		delete(bp.targetHeightToBundles, height)
	}

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
