// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import "math/big"

type (
	contractStakingDelta struct {
		cache *contractStakingCache // easy to query buckets

		bucketTypeDeltaState map[uint64]deltaState
		bucketInfoDeltaState map[uint64]deltaState
	}
)

func newContractStakingDelta() *contractStakingDelta {
	return &contractStakingDelta{
		cache:                newContractStakingCache(""),
		bucketTypeDeltaState: make(map[uint64]deltaState),
		bucketInfoDeltaState: make(map[uint64]deltaState),
	}
}

func (s *contractStakingDelta) PutHeight(height uint64) {
	s.cache.PutHeight(height)
}

func (s *contractStakingDelta) GetHeight() uint64 {
	return s.cache.Height()
}

func (s *contractStakingDelta) PutTotalBucketCount(count uint64) {
	s.cache.PutTotalBucketCount(count)
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
	if state, ok := s.bucketInfoDeltaState[id]; ok {
		switch state {
		case deltaStateAdded, deltaStateModified:
			return s.cache.MustGetBucketInfo(id), state
		default:
			return nil, state
		}
	}
	return nil, deltaStateReverted
}

func (s *contractStakingDelta) GetBucketType(id uint64) (*BucketType, deltaState) {
	if state, ok := s.bucketTypeDeltaState[id]; ok {
		switch state {
		case deltaStateAdded, deltaStateModified:
			return s.cache.MustGetBucketType(id), state
		default:
			return nil, state
		}
	}
	return nil, deltaStateReverted
}

func (s *contractStakingDelta) AddBucketInfo(id uint64, bi *bucketInfo) error {
	return s.addBucketInfo(id, bi)
}

func (s *contractStakingDelta) AddBucketType(id uint64, bt *BucketType) error {
	if _, ok := s.bucketTypeDeltaState[id]; !ok {
		s.bucketTypeDeltaState[id] = deltaStateAdded
	} else {
		var err error
		s.bucketTypeDeltaState[id], err = s.bucketTypeDeltaState[id].Transfer(deltaActionAdd)
		if err != nil {
			return err
		}
	}
	s.cache.PutBucketType(id, bt)
	return nil
}

func (s *contractStakingDelta) UpdateBucketType(id uint64, bt *BucketType) error {
	if _, ok := s.bucketTypeDeltaState[id]; !ok {
		s.bucketTypeDeltaState[id] = deltaStateModified
	} else {
		var err error
		s.bucketTypeDeltaState[id], err = s.bucketTypeDeltaState[id].Transfer(deltaActionModify)
		if err != nil {
			return err
		}
	}
	s.cache.PutBucketType(id, bt)
	return nil
}

func (s *contractStakingDelta) UpdateBucketInfo(id uint64, bi *bucketInfo) error {
	if _, ok := s.bucketInfoDeltaState[id]; !ok {
		s.bucketInfoDeltaState[id] = deltaStateModified
	} else {
		var err error
		s.bucketInfoDeltaState[id], err = s.bucketInfoDeltaState[id].Transfer(deltaActionModify)
		if err != nil {
			return err
		}
	}
	s.cache.PutBucketInfo(id, bi)
	return nil
}

func (s *contractStakingDelta) DeleteBucketInfo(id uint64) error {
	if _, ok := s.bucketInfoDeltaState[id]; !ok {
		s.bucketInfoDeltaState[id] = deltaStateRemoved
	} else {
		var err error
		s.bucketInfoDeltaState[id], err = s.bucketInfoDeltaState[id].Transfer(deltaActionRemove)
		if err != nil {
			return err
		}
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

func (s *contractStakingDelta) isBucketDeleted(id uint64) bool {
	if _, ok := s.bucketInfoDeltaState[id]; ok {
		return s.bucketInfoDeltaState[id] == deltaStateRemoved
	}
	return false
}

func (s *contractStakingDelta) addBucketInfo(id uint64, bi *bucketInfo) error {
	var err error
	if _, ok := s.bucketInfoDeltaState[id]; !ok {
		s.bucketInfoDeltaState[id] = deltaStateAdded
	} else {
		s.bucketInfoDeltaState[id], err = s.bucketInfoDeltaState[id].Transfer(deltaActionAdd)
		if err != nil {
			return err
		}
	}
	s.cache.PutBucketInfo(id, bi)
	return nil
}
