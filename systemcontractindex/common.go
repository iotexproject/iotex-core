package systemcontractindex

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

// IndexerCommon is the common struct for all contract indexers
// It provides the basic functions, including
//  1. kvstore
//  2. put/get index height
//  3. contract address
type IndexerCommon struct {
	kvstore         db.KVStore
	ns              string
	key             []byte
	startHeight     uint64
	contractAddress string
}

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

// KVStore returns the kvstore
func (s *IndexerCommon) KVStore() db.KVStore { return s.kvstore }

// ContractAddress returns the contract address
func (s *IndexerCommon) ContractAddress() string { return s.contractAddress }

// Height returns the tip block height
func (s *IndexerCommon) Height() (uint64, error) {
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

// PutHeight puts the tip block height
func (s *IndexerCommon) PutHeight(height uint64) error {
	return s.kvstore.Put(s.ns, s.key, byteutil.Uint64ToBytesBigEndian(height))
}

// BlockContinuity checks the block continuity
func (s *IndexerCommon) BlockContinuity(height uint64) (existed bool, err error) {
	expectHeight, err := s.Height()
	if err != nil {
		return false, err
	}
	if expectHeight < s.startHeight {
		expectHeight = s.startHeight
	}
	if expectHeight >= height {
		return expectHeight > height, nil
	}
	return false, errors.Errorf("invalid block height %d, expect %d", height, expectHeight)
}
