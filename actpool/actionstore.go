package actpool

import (
	"encoding/hex"
	"fmt"
	"os"
	"sync"

	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/billy"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
)

const (
	// blobSize is the protocol constrained byte size of a single blob in a
	// transaction. There can be multiple of these embedded into a single tx.
	blobSize = params.BlobTxFieldElementsPerBlob * params.BlobTxBytesPerFieldElement

	// maxBlobsPerTransaction is the maximum number of blobs a single transaction
	// is allowed to contain. Whilst the spec states it's unlimited, the block
	// data slots are protocol bound, which implicitly also limit this.
	maxBlobsPerTransaction = params.MaxBlobGasPerBlock / params.BlobTxBlobGasPerBlob

	// txAvgSize is an approximate byte size of a transaction metadata to avoid
	// tiny overflows causing all txs to move a shelf higher, wasting disk space.
	txAvgSize = 4 * 1024

	// txMaxSize is the maximum size a single transaction can have, outside
	// the included blobs. Since blob transactions are pulled instead of pushed,
	// and only a small metadata is kept in ram, the rest is on disk, there is
	// no critical limit that should be enforced. Still, capping it to some sane
	// limit can never hurt.
	txMaxSize = 1024 * 1024
)

type (
	actionStore struct {
		lifecycle.Readiness
		config actionStoreConfig // Configuration for the blob store

		store  billy.Database // Persistent data store for the tx
		stored uint64         // Useful data size of all transactions on disk

		lookup map[hash.Hash256]uint64 // Lookup table mapping hashes to tx billy entries
		lock   sync.RWMutex            // Mutex protecting the store

		encode encodeAction // Encoder for the tx
		decode decodeAction // Decoder for the tx
	}
	actionStoreConfig struct {
		Datadir string `yaml:"datadir"` // Data directory containing the currently executable blobs
		Datacap uint64 `yaml:"datacap"` // Soft-cap of database storage (hard cap is larger due to overhead)
	}

	onAction     func(selp *action.SealedEnvelope) error
	encodeAction func(selp *action.SealedEnvelope) ([]byte, error)
	decodeAction func([]byte) (*action.SealedEnvelope, error)
)

var (
	errBlobNotFound = fmt.Errorf("blob not found")
	errStoreNotOpen = fmt.Errorf("blob store is not open")
)

var defaultActionStoreConfig = actionStoreConfig{
	Datadir: "actionstore",
	Datacap: 10 * 1024 * 1024 * 1024,
}

func newBlobStore(cfg actionStoreConfig, encode encodeAction, decode decodeAction) (*actionStore, error) {
	if len(cfg.Datadir) == 0 {
		return nil, errors.New("datadir is empty")
	}
	return &actionStore{
		config: cfg,
		lookup: make(map[hash.Hash256]uint64),
		encode: encode,
		decode: decode,
	}, nil
}

func (s *actionStore) Open(onData onAction) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	dir := s.config.Datadir
	if err := os.MkdirAll(dir, 0700); err != nil {
		return errors.Wrap(err, "failed to create blob store directory")
	}
	// Index all transactions on disk and delete anything inprocessable
	var fails []uint64
	index := func(id uint64, size uint32, blob []byte) {
		act, err := s.decode(blob)
		if err != nil {
			fails = append(fails, id)
			log.L().Warn("Failed to decode action", zap.Error(err))
			return
		}
		if err = onData(act); err != nil {
			fails = append(fails, id)
			log.L().Warn("Failed to process action", zap.Error(err))
			return
		}
		s.stored += uint64(size)
		h, _ := act.Hash()
		s.lookup[h] = id
	}
	store, err := billy.Open(billy.Options{Path: dir}, newSlotter(), index)
	if err != nil {
		return errors.Wrap(err, "failed to open blob store")
	}
	s.store = store

	if len(fails) > 0 {
		log.L().Warn("Dropping invalidated blob transactions", zap.Int("count", len(fails)))

		for _, id := range fails {
			if err := s.store.Delete(id); err != nil {
				s.Close()
				return errors.Wrap(err, "failed to delete blob from store")
			}
		}
	}

	return s.TurnOn()
}

func (s *actionStore) Close() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if err := s.TurnOff(); err != nil {
		return err
	}
	return s.store.Close()
}

func (s *actionStore) Get(hash hash.Hash256) (*action.SealedEnvelope, error) {
	if !s.IsReady() {
		return nil, errors.Wrap(errStoreNotOpen, "")
	}
	s.lock.RLock()
	defer s.lock.RUnlock()

	id, ok := s.lookup[hash]
	if !ok {
		return nil, errors.Wrap(errBlobNotFound, "")
	}
	blob, err := s.store.Get(id)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get blob from store")
	}
	return s.decode(blob)
}

func (s *actionStore) Put(act *action.SealedEnvelope) error {
	if !s.IsReady() {
		return errors.Wrap(errStoreNotOpen, "")
	}
	s.lock.Lock()
	defer s.lock.Unlock()

	h, _ := act.Hash()
	// if action is already stored, nothing to do
	if _, ok := s.lookup[h]; ok {
		return nil
	}
	// insert it into the database and update the indices
	blob, err := s.encode(act)
	if err != nil {
		return errors.Wrap(err, "failed to encode action")
	}
	id, err := s.store.Put(blob)
	if err != nil {
		return errors.Wrap(err, "failed to put blob into store")
	}
	s.stored += uint64(len(blob))
	s.lookup[h] = id
	// if the datacap is exceeded, remove old data
	if s.stored > s.config.Datacap {
		s.drop()
	}
	return nil
}

func (s *actionStore) Delete(hash hash.Hash256) error {
	if !s.IsReady() {
		return errors.Wrap(errStoreNotOpen, "")
	}
	s.lock.Lock()
	defer s.lock.Unlock()

	id, ok := s.lookup[hash]
	if !ok {
		return nil
	}
	if err := s.store.Delete(id); err != nil {
		return errors.Wrap(err, "failed to delete blob from store")
	}
	delete(s.lookup, hash)
	return nil
}

// Range iterates over all stored with hashes
func (s *actionStore) Range(fn func(hash.Hash256) bool) {
	if !s.IsReady() {
		return
	}
	s.lock.RLock()
	defer s.lock.RUnlock()

	for h := range s.lookup {
		if !fn(h) {
			return
		}
	}
}

func (s *actionStore) drop() {
	h, ok := s.evict()
	if !ok {
		log.L().Debug("no worst action found")
		return
	}
	id, ok := s.lookup[h]
	if !ok {
		log.L().Warn("worst action not found in lookup", zap.String("hash", hex.EncodeToString(h[:])))
		return
	}
	if err := s.store.Delete(id); err != nil {
		log.L().Error("failed to delete worst action", zap.Error(err))
	}
	delete(s.lookup, h)
	return
}

// TODO: implement a proper eviction policy
func (s *actionStore) evict() (hash.Hash256, bool) {
	for h := range s.lookup {
		return h, true
	}
	return hash.ZeroHash256, false
}

// newSlotter creates a helper method for the Billy datastore that returns the
// individual shelf sizes used to store transactions in.
//
// The slotter will create shelves for each possible blob count + some tx metadata
// wiggle room, up to the max permitted limits.
//
// The slotter also creates a shelf for 0-blob transactions. Whilst those are not
// allowed in the current protocol, having an empty shelf is not a relevant use
// of resources, but it makes stress testing with junk transactions simpler.
func newSlotter() func() (uint32, bool) {
	slotsize := uint32(txAvgSize)
	slotsize -= uint32(blobSize) // underflows, it's ok, will overflow back in the first return

	return func() (size uint32, done bool) {
		slotsize += blobSize
		finished := slotsize > maxBlobsPerTransaction*blobSize+txMaxSize

		return slotsize, finished
	}
}
