// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

var (
	// StakingBucketsNamespace is a namespace to store vote buckets with epoch start height
	StakingBucketsNamespace = "stakingBuckets"
)

// BucketsIndexer is an indexer to store vote buckets by given height
type BucketsIndexer struct {
	mutex   sync.RWMutex
	kvStore db.KVStore
}

// NewStakingBucketsIndexer creates a new StakingBucketsIndexer
func NewStakingBucketsIndexer(kv db.KVStore) (*BucketsIndexer, error) {
	if kv == nil {
		return nil, errors.New("empty kvStore")
	}
	x := BucketsIndexer{
		kvStore: kv,
	}
	return &x, nil
}

// Start starts the indexer
func (vb *BucketsIndexer) Start(ctx context.Context) error {
	return vb.kvStore.Start(ctx)
}

// Stop stops the indexer
func (vb *BucketsIndexer) Stop(ctx context.Context) error {
	return vb.kvStore.Stop(ctx)
}

// Put puts vote buckets into indexer
func (vb *BucketsIndexer) Put(height uint64, buckets *iotextypes.VoteBucketList) error {
	vb.mutex.Lock()
	defer vb.mutex.Unlock()
	bucketsBytes, err := proto.Marshal(buckets)
	if err != nil {
		return err
	}
	return vb.kvStore.Put(StakingBucketsNamespace, byteutil.Uint64ToBytes(height), bucketsBytes)
}

// Get gets vote buckets from indexer given epoch start height
func (vb *BucketsIndexer) Get(height uint64, offset, limit uint32) ([]byte, error) {
	vb.mutex.RLock()
	defer vb.mutex.RUnlock()
	buckets := &iotextypes.VoteBucketList{}
	ret, err := vb.kvStore.Get(StakingBucketsNamespace, byteutil.Uint64ToBytes(height))
	if errors.Cause(err) == db.ErrNotExist {
		return proto.Marshal(buckets)
	}
	if err != nil {
		return nil, err
	}
	if err := proto.Unmarshal(ret, buckets); err != nil {
		return nil, err
	}
	length := uint32(len(buckets.Buckets))
	if offset >= length {
		return proto.Marshal(&iotextypes.VoteBucketList{})
	}
	end := offset + limit
	if end > uint32(len(buckets.Buckets)) {
		end = uint32(len(buckets.Buckets))
	}
	buckets.Buckets = buckets.Buckets[offset:end]
	return proto.Marshal(buckets)
}
