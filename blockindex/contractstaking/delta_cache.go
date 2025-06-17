// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"math/big"
)

type (
	contractStakingDelta struct {
		cache *contractStakingCache // easy to query buckets

		bucketTypeDeltaState map[uint64]deltaState
		bucketInfoDeltaState map[uint64]deltaState
	}
)

func newContractStakingDelta() *contractStakingDelta {
	return &contractStakingDelta{
		cache:                newContractStakingCache(Config{}),
		bucketTypeDeltaState: make(map[uint64]deltaState),
		bucketInfoDeltaState: make(map[uint64]deltaState),
	}
}

func (s *contractStakingDelta) BucketInfoDelta() map[deltaState]map[uint64]*bucketInfo {
	delta := map[deltaState]map[uint64]*bucketInfo{
		deltaStateAdded:    make(map[uint64]*bucketInfo),
		deltaStateRemoved:  make(map[uint64]*bucketInfo),
		deltaStateModified: make(map[uint64]*bucketInfo),
	}
	for id, state := range s.bucketInfoDeltaState {
		switch state {
		case deltaStateAdded:
			delta[state][id] = s.cache.MustGetBucketInfo(id)
		case deltaStateRemoved:
			delta[state][id] = nil
		case deltaStateModified:
			delta[state][id] = s.cache.MustGetBucketInfo(id)
		}
	}
	return delta
}

func (s *contractStakingDelta) BucketTypeDelta() map[deltaState]map[uint64]*BucketType {
	delta := map[deltaState]map[uint64]*BucketType{
		deltaStateAdded:    make(map[uint64]*BucketType),
		deltaStateModified: make(map[uint64]*BucketType),
	}
	for id, state := range s.bucketTypeDeltaState {
		switch state {
		case deltaStateAdded:
			delta[state][id] = s.cache.MustGetBucketType(id)
		case deltaStateModified:
			delta[state][id] = s.cache.MustGetBucketType(id)
		}
	}
	return delta
}

func (s *contractStakingDelta) MustGetBucketType(id uint64) *BucketType {
	return s.cache.MustGetBucketType(id)
}

func (s *contractStakingDelta) MatchBucketType(amount *big.Int, duration uint64) (uint64, *BucketType, bool) {
	return s.cache.MatchBucketType(amount, duration)
}

func (s *contractStakingDelta) GetBucketInfo(id uint64) (*bucketInfo, deltaState) {
	state := s.bucketInfoDeltaState[id]
	switch state {
	case deltaStateAdded, deltaStateModified:
		return s.cache.MustGetBucketInfo(id), state
	default: // deltaStateRemoved, deltaStateUnchanged
		return nil, state
	}
}

func (s *contractStakingDelta) GetBucketType(id uint64) (*BucketType, deltaState) {
	state := s.bucketTypeDeltaState[id]
	switch state {
	case deltaStateAdded, deltaStateModified:
		return s.cache.MustGetBucketType(id), state
	default: // deltaStateUnchanged
		return nil, state
	}
}

func (s *contractStakingDelta) AddBucketInfo(id uint64, bi *bucketInfo) error {
	return s.addBucketInfo(id, bi)
}

func (s *contractStakingDelta) AddBucketType(id uint64, bt *BucketType) error {
	var err error
	s.bucketTypeDeltaState[id], err = s.bucketTypeDeltaState[id].Transfer(deltaActionAdd)
	if err != nil {
		return err
	}

	s.cache.PutBucketType(id, bt)
	return nil
}

func (s *contractStakingDelta) UpdateBucketType(id uint64, bt *BucketType) error {
	var err error
	s.bucketTypeDeltaState[id], err = s.bucketTypeDeltaState[id].Transfer(deltaActionModify)
	if err != nil {
		return err
	}
	s.cache.PutBucketType(id, bt)
	return nil
}

func (s *contractStakingDelta) UpdateBucketInfo(id uint64, bi *bucketInfo) error {
	var err error
	s.bucketInfoDeltaState[id], err = s.bucketInfoDeltaState[id].Transfer(deltaActionModify)
	if err != nil {
		return err
	}
	s.cache.PutBucketInfo(id, bi)
	return nil
}

func (s *contractStakingDelta) DeleteBucketInfo(id uint64) error {
	var err error
	s.bucketInfoDeltaState[id], err = s.bucketInfoDeltaState[id].Transfer(deltaActionRemove)
	if err != nil {
		return err
	}
	s.cache.DeleteBucketInfo(id)
	return nil
}

func (s *contractStakingDelta) AddedBucketCnt() uint64 {
	addedBucketCnt := uint64(0)
	for _, state := range s.bucketInfoDeltaState {
		if state == deltaStateAdded {
			addedBucketCnt++
		}
	}
	return addedBucketCnt
}

func (s *contractStakingDelta) AddedBucketTypeCnt() uint64 {
	cnt := uint64(0)
	for _, state := range s.bucketTypeDeltaState {
		if state == deltaStateAdded {
			cnt++
		}
	}
	return cnt
}

func (s *contractStakingDelta) addBucketInfo(id uint64, bi *bucketInfo) error {
	var err error
	s.bucketInfoDeltaState[id], err = s.bucketInfoDeltaState[id].Transfer(deltaActionAdd)
	if err != nil {
		return err
	}
	s.cache.PutBucketInfo(id, bi)
	return nil
}
