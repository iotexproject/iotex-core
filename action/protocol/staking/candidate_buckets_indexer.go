// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const (
	// StakingCandidatesNamespace is a namespace to store candidates with epoch start height
	StakingCandidatesNamespace = "stakingCandidates"
	// StakingBucketsNamespace is a namespace to store vote buckets with epoch start height
	StakingBucketsNamespace = "stakingBuckets"
	// StakingMetaNamespace is a namespace to store metadata
	StakingMetaNamespace = "stakingMeta"
)

var (
	_candHeightKey        = []byte("cht")
	_bucketHeightKey      = []byte("bht")
	_latestCandidatesHash = []byte("lch")
	_latestBucketsHash    = []byte("lbh")
)

// CandidatesBucketsIndexer is an indexer to store candidates by given height
type CandidatesBucketsIndexer struct {
	latestCandidatesHeight uint64
	latestBucketsHeight    uint64
	latestCandidatesHash   hash.Hash160
	latestBucketsHash      hash.Hash160
	kvStore                db.KVStoreForRangeIndex
}

// NewStakingCandidatesBucketsIndexer creates a new StakingCandidatesIndexer
func NewStakingCandidatesBucketsIndexer(kv db.KVStoreForRangeIndex) (*CandidatesBucketsIndexer, error) {
	if kv == nil {
		return nil, ErrMissingField
	}
	return &CandidatesBucketsIndexer{
		kvStore: kv,
	}, nil
}

// Start starts the indexer
func (cbi *CandidatesBucketsIndexer) Start(ctx context.Context) error {
	if err := cbi.kvStore.Start(ctx); err != nil {
		return err
	}
	ret, err := cbi.kvStore.Get(StakingMetaNamespace, _candHeightKey)
	switch errors.Cause(err) {
	case nil:
		cbi.latestCandidatesHeight = byteutil.BytesToUint64BigEndian(ret)
	case db.ErrNotExist:
		cbi.latestCandidatesHeight = 0
	default:
		return err
	}

	ret, err = cbi.kvStore.Get(StakingMetaNamespace, _latestCandidatesHash)
	switch errors.Cause(err) {
	case nil:
		cbi.latestCandidatesHash = hash.BytesToHash160(ret)
	case db.ErrNotExist:
		cbi.latestCandidatesHash = hash.ZeroHash160
	default:
		return err
	}

	ret, err = cbi.kvStore.Get(StakingMetaNamespace, _bucketHeightKey)
	switch errors.Cause(err) {
	case nil:
		cbi.latestBucketsHeight = byteutil.BytesToUint64BigEndian(ret)
	case db.ErrNotExist:
		cbi.latestBucketsHeight = 0
	default:
		return err
	}

	ret, err = cbi.kvStore.Get(StakingMetaNamespace, _latestBucketsHash)
	switch errors.Cause(err) {
	case nil:
		cbi.latestBucketsHash = hash.BytesToHash160(ret)
	case db.ErrNotExist:
		cbi.latestBucketsHash = hash.ZeroHash160
	default:
		return err
	}
	return nil
}

// Stop stops the indexer
func (cbi *CandidatesBucketsIndexer) Stop(ctx context.Context) error {
	return cbi.kvStore.Stop(ctx)
}

// PutCandidates puts candidates into indexer
func (cbi *CandidatesBucketsIndexer) PutCandidates(height uint64, candidates *iotextypes.CandidateListV2) error {
	candidatesBytes, err := proto.Marshal(candidates)
	if err != nil {
		return err
	}

	if err := cbi.putToIndexer(StakingCandidatesNamespace, height, candidatesBytes); err != nil {
		return err
	}
	cbi.latestCandidatesHeight = height
	return nil
}

// GetCandidates gets candidates from indexer given epoch start height
func (cbi *CandidatesBucketsIndexer) GetCandidates(height uint64, offset, limit uint32) ([]byte, uint64, error) {
	if height > cbi.latestCandidatesHeight {
		height = cbi.latestCandidatesHeight
	}
	candidateList := &iotextypes.CandidateListV2{}
	ret, err := getFromIndexer(cbi.kvStore, StakingCandidatesNamespace, height)
	cause := errors.Cause(err)
	if cause == db.ErrNotExist || cause == db.ErrBucketNotExist {
		d, err := proto.Marshal(candidateList)
		return d, height, err
	}
	if err != nil {
		return nil, height, err
	}
	if err := proto.Unmarshal(ret, candidateList); err != nil {
		return nil, height, err
	}
	length := uint32(len(candidateList.Candidates))
	if offset >= length {
		d, err := proto.Marshal(&iotextypes.CandidateListV2{})
		return d, height, err
	}
	end := offset + limit
	if end > uint32(len(candidateList.Candidates)) {
		end = uint32(len(candidateList.Candidates))
	}
	candidateList.Candidates = candidateList.Candidates[offset:end]
	d, err := proto.Marshal(candidateList)
	return d, height, err
}

// PutBuckets puts vote buckets into indexer
func (cbi *CandidatesBucketsIndexer) PutBuckets(height uint64, buckets *iotextypes.VoteBucketList) error {
	bucketsBytes, err := proto.Marshal(buckets)
	if err != nil {
		return err
	}

	if err := cbi.putToIndexer(StakingBucketsNamespace, height, bucketsBytes); err != nil {
		return err
	}
	cbi.latestBucketsHeight = height
	return nil
}

// GetBuckets gets vote buckets from indexer given epoch start height
func (cbi *CandidatesBucketsIndexer) GetBuckets(height uint64, offset, limit uint32) ([]byte, uint64, error) {
	if height > cbi.latestBucketsHeight {
		height = cbi.latestBucketsHeight
	}
	buckets := &iotextypes.VoteBucketList{}
	ret, err := getFromIndexer(cbi.kvStore, StakingBucketsNamespace, height)
	cause := errors.Cause(err)
	if cause == db.ErrNotExist || cause == db.ErrBucketNotExist {
		d, err := proto.Marshal(buckets)
		return d, height, err
	}
	if err != nil {
		return nil, height, err
	}
	if err := proto.Unmarshal(ret, buckets); err != nil {
		return nil, height, err
	}
	length := uint32(len(buckets.Buckets))
	if offset >= length {
		d, err := proto.Marshal(&iotextypes.VoteBucketList{})
		return d, height, err
	}
	end := offset + limit
	if end > uint32(len(buckets.Buckets)) {
		end = uint32(len(buckets.Buckets))
	}
	buckets.Buckets = buckets.Buckets[offset:end]
	d, err := proto.Marshal(buckets)
	return d, height, err
}

func (cbi *CandidatesBucketsIndexer) putToIndexer(ns string, height uint64, data []byte) error {
	var (
		h          = hash.Hash160b(data)
		dataExist  bool
		heightKey  []byte
		latestHash []byte
	)
	switch ns {
	case StakingCandidatesNamespace:
		dataExist = (h == cbi.latestCandidatesHash)
		heightKey = _candHeightKey
		latestHash = _latestCandidatesHash
	case StakingBucketsNamespace:
		dataExist = (h == cbi.latestBucketsHash)
		heightKey = _bucketHeightKey
		latestHash = _latestBucketsHash
	default:
		return ErrTypeAssertion
	}

	heightBytes := byteutil.Uint64ToBytesBigEndian(height)
	if dataExist {
		// same bytes already exist, do nothing
		return cbi.kvStore.Put(StakingMetaNamespace, heightKey, heightBytes)
	}

	// update latest height
	b := batch.NewBatch()
	b.Put(ns, heightBytes, data, "failed to write data bytes")
	b.Put(StakingMetaNamespace, heightKey, heightBytes, "failed to update indexer height")
	b.Put(StakingMetaNamespace, latestHash, h[:], "failed to update latest hash")
	if err := cbi.kvStore.WriteBatch(b); err != nil {
		return err
	}
	// update latest hash
	if ns == StakingCandidatesNamespace {
		cbi.latestCandidatesHash = h
	} else {
		cbi.latestBucketsHash = h
	}
	return nil
}

func getFromIndexer(kv db.KVStoreForRangeIndex, ns string, height uint64) ([]byte, error) {
	b, err := kv.Get(ns, byteutil.Uint64ToBytesBigEndian(height))
	switch errors.Cause(err) {
	case nil:
		return b, nil
	case db.ErrNotExist:
		// height does not exist, fallback to previous height
		return kv.SeekPrev([]byte(ns), height)
	default:
		return nil, err
	}
}
