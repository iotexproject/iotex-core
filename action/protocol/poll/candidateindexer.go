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
	CandidateNamespace = "candidates"
	KickoutNamespace   = "kickout"
	ErrIndexerNotExist = errors.New("not exist in DB")
)

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

func (cd *CandidateIndexer) PutCandidate(height uint64, candidates *state.CandidateList) error {
	cd.mutex.Lock()
	defer cd.mutex.Unlock()
	candidatesByte, err := candidates.Serialize()
	if err != nil {
		return err
	}
	return cd.kvStore.Put(CandidateNamespace, byteutil.Uint64ToBytes(height), candidatesByte)
}

func (cd *CandidateIndexer) PutKickoutList(height uint64, kickoutList *vote.Blacklist) error {
	cd.mutex.Lock()
	defer cd.mutex.Unlock()
	kickoutListByte, err := kickoutList.Serialize()
	if err != nil {
		return err
	}
	return cd.kvStore.Put(KickoutNamespace, byteutil.Uint64ToBytes(height), kickoutListByte)
}

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
