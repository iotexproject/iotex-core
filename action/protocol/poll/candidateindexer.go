// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

var (
	// CandidateNamespace is a namespace to store raw candidate
	CandidateNamespace = "candidates"
	// KickoutNamespace is a namespace to store kickoutlist
	KickoutNamespace = "kickout"
	// ErrIndexerNotExist is an error that shows not exist in candidate indexer DB
	ErrIndexerNotExist = errors.New("not exist in DB")
)

// CandidateIndexer is an indexer to store candidate/blacklist by given height
type CandidateIndexer struct {
	mutex   sync.RWMutex
	kvStore db.KVStore
}

// NewCandidateIndexer creates a new CandidateIndexer
func NewCandidateIndexer(kv db.KVStore) (*CandidateIndexer, error) {
	if kv == nil {
		return nil, errors.New("empty kvStore")
	}
	x := CandidateIndexer{
		kvStore: kv,
	}
	return &x, nil
}

// PutCandidate puts candidate list into indexer
func (cd *CandidateIndexer) PutCandidate(height uint64, candidates *state.CandidateList) error {
	cd.mutex.Lock()
	defer cd.mutex.Unlock()
	candidatesByte, err := candidates.Serialize()
	if err != nil {
		return err
	}
	return cd.kvStore.Put(CandidateNamespace, byteutil.Uint64ToBytes(height), candidatesByte)
}

// PutKickoutList puts kickout list into indexer
func (cd *CandidateIndexer) PutKickoutList(height uint64, kickoutList *vote.Blacklist) error {
	cd.mutex.Lock()
	defer cd.mutex.Unlock()
	kickoutListByte, err := kickoutList.Serialize()
	if err != nil {
		return err
	}
	return cd.kvStore.Put(KickoutNamespace, byteutil.Uint64ToBytes(height), kickoutListByte)
}

// GetCandidate gets candidate list from indexer given epoch start height
func (cd *CandidateIndexer) GetCandidate(height uint64) (state.CandidateList, error) {
	cd.mutex.RLock()
	defer cd.mutex.RUnlock()
	candidates := &state.CandidateList{}
	bytes, err := cd.kvStore.Get(CandidateNamespace, byteutil.Uint64ToBytes(height))
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			return nil, ErrIndexerNotExist
		}
		return nil, err
	}
	if err := candidates.Deserialize(bytes); err != nil {
		return nil, err
	}
	return *candidates, nil
}

// GetKickoutList gets kickout list from indexer given epoch start height
func (cd *CandidateIndexer) GetKickoutList(height uint64) (*vote.Blacklist, error) {
	cd.mutex.RLock()
	defer cd.mutex.RUnlock()
	bl := &vote.Blacklist{}
	bytes, err := cd.kvStore.Get(KickoutNamespace, byteutil.Uint64ToBytes(height))
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			return nil, ErrIndexerNotExist
		}
		return nil, err
	}
	if err := bl.Deserialize(bytes); err != nil {
		return nil, err
	}
	return bl, nil
}
