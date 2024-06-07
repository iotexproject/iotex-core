package systemcontractindex

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

type (
	// IndexerCommon is the common struct for all contract indexers
	// It provides the basic functions, including
	//  1. kvstore
	//  2. put/get index height
	//  3. contract address
	IndexerCommon struct {
		kvstore         db.KVStore
		ns              string
		key             []byte
		startHeight     uint64
		height          uint64
		contractAddress string
	}

	stateType interface {
		Load(kvstore db.KVStore) error
	}
	kvStoreWithVersion interface {
		db.KVStore
		WithVersion(version uint64) db.KVStore
	}
)

// NewIndexerCommon creates a new IndexerCommon
func NewIndexerCommon(kvstore db.KVStore, ns string, key []byte, contractAddress string, startHeight uint64) *IndexerCommon {
	return &IndexerCommon{
		kvstore:         kvstore,
		ns:              ns,
		key:             key,
		startHeight:     startHeight,
		contractAddress: contractAddress,
	}
}

// Start starts the indexer
func (s *IndexerCommon) Start(ctx context.Context) error {
	if err := s.kvstore.Start(ctx); err != nil {
		return err
	}
	h, err := s.loadHeight()
	if err != nil {
		return err
	}
	s.height = h
	return nil
}

// Stop stops the indexer
func (s *IndexerCommon) Stop(ctx context.Context) error {
	return s.kvstore.Stop(ctx)
}

// StateAt loads the state at the given height
func (s *IndexerCommon) StateAt(state stateType, height uint64) error {
	if kvstore, ok := s.kvstore.(kvStoreWithVersion); ok {
		return state.Load(kvstore.WithVersion(height))
	}
	return errors.New("kvstore does not support versioning")
}

// ContractAddress returns the contract address
func (s *IndexerCommon) ContractAddress() string { return s.contractAddress }

// Height returns the tip block height
func (s *IndexerCommon) Height() uint64 {
	return s.height
}

func (s *IndexerCommon) loadHeight() (uint64, error) {
	// get the tip block height
	var height uint64
	h, err := s.kvstore.Get(s.ns, s.key)
	if err != nil {
		if !errors.Is(err, db.ErrNotExist) {
			return 0, err
		}
		height = 0
	} else {
		height = byteutil.BytesToUint64BigEndian(h)
	}
	return height, nil
}

// StartHeight returns the start height of the indexer
func (s *IndexerCommon) StartHeight() uint64 { return s.startHeight }

// Commit commits the height to the indexer
func (s *IndexerCommon) Commit(height uint64, delta batch.KVStoreBatch) error {
	s.height = height
	delta.Put(s.ns, s.key, byteutil.Uint64ToBytesBigEndian(height), "failed to put height")
	return s.kvstore.WriteBatch(delta)
}

// ExpectedHeight returns the expected height
func (s *IndexerCommon) ExpectedHeight() uint64 {
	if s.height < s.startHeight {
		return s.startHeight
	}
	return s.height + 1
}
