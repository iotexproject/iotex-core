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

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const (
	// StakingCandidatesNamespace is a namespace to store candidates with epoch start height
	StakingCandidatesNamespace = "stakingCandidates"
	// StakingBucketsNamespace is a namespace to store vote buckets with epoch start height
	StakingBucketsNamespace = "stakingBuckets"
)

const indexerHeightKey = "latestHeight"

// CandidatesBucketsIndexer is an indexer to store candidates by given height
type CandidatesBucketsIndexer struct {
	latestCandidatesHeight uint64
	latestBucketsHeight    uint64
	kvStore                db.KVStore
}

// NewStakingCandidatesBucketsIndexer creates a new StakingCandidatesIndexer
func NewStakingCandidatesBucketsIndexer(kv db.KVStore) (*CandidatesBucketsIndexer, error) {
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
	ret, err := cbi.kvStore.Get(StakingCandidatesNamespace, []byte(indexerHeightKey))
	switch errors.Cause(err) {
	case nil:
		cbi.latestCandidatesHeight = byteutil.BytesToUint64BigEndian(ret)
	case db.ErrNotExist:
		cbi.latestCandidatesHeight = 0
	default:
		return err
	}

	ret, err = cbi.kvStore.Get(StakingBucketsNamespace, []byte(indexerHeightKey))
	switch errors.Cause(err) {
	case nil:
		cbi.latestBucketsHeight = byteutil.BytesToUint64BigEndian(ret)
	case db.ErrNotExist:
		cbi.latestBucketsHeight = 0
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
	heightBytes := byteutil.Uint64ToBytesBigEndian(height)
	if err := cbi.kvStore.Put(StakingCandidatesNamespace, heightBytes, candidatesBytes); err != nil {
		return err
	}
	if err := cbi.kvStore.Put(StakingCandidatesNamespace, []byte(indexerHeightKey), heightBytes); err != nil {
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
	ret, err := cbi.kvStore.Get(StakingCandidatesNamespace, byteutil.Uint64ToBytesBigEndian(height))
	if errors.Cause(err) == db.ErrNotExist {
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
	heightBytes := byteutil.Uint64ToBytesBigEndian(height)
	if err := cbi.kvStore.Put(StakingBucketsNamespace, heightBytes, bucketsBytes); err != nil {
		return err
	}
	if err := cbi.kvStore.Put(StakingBucketsNamespace, []byte(indexerHeightKey), heightBytes); err != nil {
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
	ret, err := cbi.kvStore.Get(StakingBucketsNamespace, byteutil.Uint64ToBytesBigEndian(height))
	if errors.Cause(err) == db.ErrNotExist {
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
