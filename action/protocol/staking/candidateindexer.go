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
	// StakingCandidatesNamespace is a namespace to store candidates with epoch start height
	StakingCandidatesNamespace = "stakingCandidates"
)

// CandidatesIndexer is an indexer to store candidates by given height
type CandidatesIndexer struct {
	mutex   sync.RWMutex
	kvStore db.KVStore
}

// NewStakingCandidatesIndexer creates a new StakingCandidatesIndexer
func NewStakingCandidatesIndexer(kv db.KVStore) (*CandidatesIndexer, error) {
	if kv == nil {
		return nil, errors.New("empty kvStore")
	}
	x := CandidatesIndexer{
		kvStore: kv,
	}
	return &x, nil
}

// Start starts the indexer
func (vb *CandidatesIndexer) Start(ctx context.Context) error {
	return vb.kvStore.Start(ctx)
}

// Stop stops the indexer
func (vb *CandidatesIndexer) Stop(ctx context.Context) error {
	return vb.kvStore.Stop(ctx)
}

// Put puts vote buckets into indexer
func (vb *CandidatesIndexer) Put(height uint64, candidates *iotextypes.CandidateListV2) error {
	vb.mutex.Lock()
	defer vb.mutex.Unlock()
	candidatesBytes, err := proto.Marshal(candidates)
	if err != nil {
		return err
	}
	return vb.kvStore.Put(StakingCandidatesNamespace, byteutil.Uint64ToBytes(height), candidatesBytes)
}

// Get gets vote buckets from indexer given epoch start height
func (vb *CandidatesIndexer) Get(height uint64, offset, limit uint32) ([]byte, error) {
	vb.mutex.RLock()
	defer vb.mutex.RUnlock()
	candidateList := &iotextypes.CandidateListV2{}
	ret, err := vb.kvStore.Get(StakingCandidatesNamespace, byteutil.Uint64ToBytes(height))
	if errors.Cause(err) == db.ErrNotExist {
		return proto.Marshal(candidateList)
	}
	if err != nil {
		return nil, err
	}
	if err := proto.Unmarshal(ret, candidateList); err != nil {
		return nil, err
	}
	length := uint32(len(candidateList.Candidates))
	if offset >= length {
		return proto.Marshal(&iotextypes.CandidateListV2{})
	}
	end := offset + limit
	if end > uint32(len(candidateList.Candidates)) {
		end = uint32(len(candidateList.Candidates))
	}
	candidateList.Candidates = candidateList.Candidates[offset:end]
	return proto.Marshal(candidateList)
}
