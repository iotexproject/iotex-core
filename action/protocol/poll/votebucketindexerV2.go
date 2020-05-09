// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

var (
	// VoteBucketV2Namespace is a namespace to store vote buckets with epoch start height
	VoteBucketV2Namespace = "votebucketV2"
)

// VoteBucketV2Indexer is an indexer to store vote buckets by given height
type VoteBucketV2Indexer struct {
	mutex   sync.RWMutex
	kvStore db.KVStore
}

// NewVoteBucketV2Indexer creates a new VoteBucketIndexer
func NewVoteBucketV2Indexer(kv db.KVStore) (*VoteBucketV2Indexer, error) {
	if kv == nil {
		return nil, errors.New("empty kvStore")
	}
	x := VoteBucketV2Indexer{
		kvStore: kv,
	}
	return &x, nil
}

// Start starts the indexer
func (vb *VoteBucketV2Indexer) Start(ctx context.Context) error {
	return vb.kvStore.Start(ctx)
}

// Stop stops the indexer
func (vb *VoteBucketV2Indexer) Stop(ctx context.Context) error {
	return vb.kvStore.Stop(ctx)
}

// Put puts vote buckets into indexer
func (vb *VoteBucketV2Indexer) Put(height uint64, buckets *iotextypes.VoteBucketList) error {
	vb.mutex.Lock()
	defer vb.mutex.Unlock()
	bucketsBytes, err := proto.Marshal(buckets)
	if err != nil {
		return err
	}
	return vb.kvStore.Put(VoteBucketV2Namespace, byteutil.Uint64ToBytes(height), bucketsBytes)
}

// Get gets vote buckets from indexer given epoch start height
func (vb *VoteBucketV2Indexer) Get(height uint64) ([]byte, error) {
	vb.mutex.RLock()
	defer vb.mutex.RUnlock()
	return vb.kvStore.Get(VoteBucketV2Namespace, byteutil.Uint64ToBytes(height))
}
