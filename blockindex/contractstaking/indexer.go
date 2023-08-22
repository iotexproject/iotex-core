// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"context"
	"encoding/hex"
	"math/big"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"golang.org/x/crypto/sha3"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const (
	maxBlockNumber uint64 = math.MaxUint64
)

type (
	// Indexer is the contract staking indexer
	// Main functions:
	// 		1. handle contract staking contract events when new block comes to generate index data
	// 		2. provide query interface for contract staking index data
	Indexer struct {
		kvstore              db.KVStore            // persistent storage, used to initialize index cache at startup
		cache                *contractStakingCache // in-memory index for clean data, used to query index data
		contractAddress      string                // stake contract address
		contractDeployHeight uint64                // height of the contract deployment
	}
)

// NewContractStakingIndexer creates a new contract staking indexer
func NewContractStakingIndexer(kvStore db.KVStore, contractAddr string, contractDeployHeight uint64) (*Indexer, error) {
	if kvStore == nil {
		return nil, errors.New("kv store is nil")
	}
	if _, err := address.FromString(contractAddr); err != nil {
		return nil, errors.Wrapf(err, "invalid contract address %s", contractAddr)
	}
	return &Indexer{
		kvstore:              kvStore,
		contractAddress:      contractAddr,
		contractDeployHeight: contractDeployHeight,
	}, nil
}

// Start starts the indexer
func (s *Indexer) Start(ctx context.Context) error {
	if err := s.kvstore.Start(ctx); err != nil {
		return err
	}
	s.cache = newContractStakingCache(s.contractAddress)
	return s.loadFromDB()
}

// Stop stops the indexer
func (s *Indexer) Stop(ctx context.Context) error {
	if err := s.kvstore.Stop(ctx); err != nil {
		return err
	}
	return nil
}

// Height returns the tip block height
func (s *Indexer) Height() (uint64, error) {
	return s.cache.Height(), nil
}

// StartHeight returns the start height of the indexer
func (s *Indexer) StartHeight() uint64 {
	return s.contractDeployHeight
}

// CandidateVotes returns the candidate votes
func (s *Indexer) CandidateVotes(candidate address.Address) *big.Int {
	return s.cache.CandidateVotes(candidate)
}

// Buckets returns the buckets
func (s *Indexer) Buckets() ([]*Bucket, error) {
	return s.cache.Buckets(), nil
}

// Bucket returns the bucket
func (s *Indexer) Bucket(id uint64) (*Bucket, bool) {
	return s.cache.Bucket(id)
}

// BucketsByIndices returns the buckets by indices
func (s *Indexer) BucketsByIndices(indices []uint64) ([]*Bucket, error) {
	return s.cache.BucketsByIndices(indices)
}

// BucketsByCandidate returns the buckets by candidate
func (s *Indexer) BucketsByCandidate(candidate address.Address) ([]*Bucket, error) {
	return s.cache.BucketsByCandidate(candidate), nil
}

// TotalBucketCount returns the total bucket count including active and burnt buckets
func (s *Indexer) TotalBucketCount() uint64 {
	return s.cache.TotalBucketCount()
}

// BucketTypes returns the active bucket types
func (s *Indexer) BucketTypes() ([]*BucketType, error) {
	btMap := s.cache.ActiveBucketTypes()
	bts := make([]*BucketType, 0, len(btMap))
	for _, bt := range btMap {
		bts = append(bts, bt)
	}
	return bts, nil
}

// PutBlock puts a block into indexer
func (s *Indexer) PutBlock(ctx context.Context, blk *block.Block) error {
	expectHeight := s.cache.Height() + 1
	if expectHeight < s.contractDeployHeight {
		expectHeight = s.contractDeployHeight
	}
	if blk.Height() < expectHeight {
		return nil
	}
	if blk.Height() > expectHeight {
		return errors.Errorf("invalid block height %d, expect %d", blk.Height(), expectHeight)
	}
	// new event handler for this block
	handler := newContractStakingEventHandler(s.cache)

	// handle events of block
	for _, receipt := range blk.Receipts {
		if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
			continue
		}
		for _, log := range receipt.Logs() {
			if log.Address != s.contractAddress {
				continue
			}
			if err := handler.HandleEvent(ctx, blk, log); err != nil {
				return err
			}
		}
	}

	// commit the result
	return s.commit(handler, blk.Height())
}

// DeleteTipBlock deletes the tip block from indexer
func (s *Indexer) DeleteTipBlock(context.Context, *block.Block) error {
	return errors.New("not implemented")
}

func (s *Indexer) commit(handler *contractStakingEventHandler, height uint64) error {
	batch, delta := handler.Result()
	// update cache
	if err := s.cache.Merge(delta, height); err != nil {
		s.reloadCache()
		return err
	}
	// update db
	batch.Put(_StakingNS, _stakingHeightKey, byteutil.Uint64ToBytesBigEndian(height), "failed to put height")
	if err := s.kvstore.WriteBatch(batch); err != nil {
		s.reloadCache()
		return err
	}
	return nil
}

func (s *Indexer) reloadCache() error {
	s.cache = newContractStakingCache(s.contractAddress)
	return s.loadFromDB()
}

func (s *Indexer) loadFromDB() error {
	return s.cache.LoadFromDB(s.kvstore)
}

// the indexer DB contains 4 items:
// 1. the height of the indexer DB
// 2. total bucket counts indexed by the DB
// 3. all buckets
// 4. all bucket types
// hash() returns a cryptographic hash of all these contents
func (s *Indexer) hash() (string, error) {
	// load height
	height, err := s.kvstore.Get(_StakingNS, _stakingHeightKey)
	if err != nil && !errors.Is(err, db.ErrNotExist) {
		return "", err
	}
	// load total bucket count
	tbc, err := s.kvstore.Get(_StakingNS, _stakingTotalBucketCountKey)
	if err != nil && !errors.Is(err, db.ErrNotExist) {
		return "", err
	}
	// load bucket info
	iks, ivs, err := s.kvstore.Filter(_StakingBucketInfoNS, func(k, v []byte) bool { return true }, nil, nil)
	if err != nil && !errors.Is(err, db.ErrBucketNotExist) {
		return "", err
	}
	// load bucket type
	tks, tvs, err := s.kvstore.Filter(_StakingBucketTypeNS, func(k, v []byte) bool { return true }, nil, nil)
	if err != nil && !errors.Is(err, db.ErrBucketNotExist) {
		return "", err
	}

	// hash all values
	h := sha3.New256()
	if len(height) > 0 {
		h.Write(height)
	}
	if len(tbc) > 0 {
		h.Write(tbc)
	}
	for i := range iks {
		h.Write(iks[i])
	}
	for i := range ivs {
		h.Write(ivs[i])
	}
	for i := range tks {
		h.Write(tks[i])
	}
	for i := range tvs {
		h.Write(tvs[i])
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// IndexerFingerPrint returns a fingerprint of the indexer DB file
func IndexerFingerPrint(dbPath string) (string, error) {
	cfg := config.Default
	dbConfig := cfg.DB
	dbConfig.DbPath = dbPath
	indexer, err := NewContractStakingIndexer(db.NewBoltDB(dbConfig), cfg.Genesis.SystemStakingContractAddress, cfg.Genesis.SystemStakingContractHeight)
	if err != nil {
		return "", err
	}

	ctx := context.Background()
	if err := indexer.kvstore.Start(ctx); err != nil {
		return "", err
	}
	defer func() {
		indexer.kvstore.Stop(ctx)
	}()
	return indexer.hash()
}
