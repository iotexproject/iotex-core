// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import "github.com/pkg/errors"

const (
	deltaStateAdded deltaState = iota
	deltaStateRemoved
	deltaStateModified
	deltaStateReverted

	deltaActionAdd deltaAction = iota
	deltaActionRemove
	deltaActionModify
)

type (
	deltaState  int
	deltaAction int

	liquidStakingDelta struct {
		liquidStakingCacheManager // easy to query buckets

		bucketTypeDeltaState map[uint64]deltaState
		bucketInfoDeltaState map[uint64]deltaState
	}
)

var (
	deltaStateTransferMap = map[deltaState]map[deltaAction]deltaState{
		deltaStateAdded: {
			deltaActionRemove: deltaStateReverted,
			deltaActionModify: deltaStateAdded,
		},
		deltaStateRemoved: {
			deltaActionAdd: deltaStateModified,
		},
		deltaStateModified: {
			deltaActionModify: deltaStateModified,
			deltaActionRemove: deltaStateRemoved,
		},
		deltaStateReverted: {
			deltaActionAdd: deltaStateAdded,
		},
	}
)

func (s deltaState) transfer(act deltaAction) (deltaState, error) {
	if _, ok := deltaStateTransferMap[s]; !ok {
		return s, errors.Errorf("invalid delta state %d", s)
	}
	if _, ok := deltaStateTransferMap[s][act]; !ok {
		return s, errors.Errorf("invalid delta action %d on state %d", act, s)
	}
	return deltaStateTransferMap[s][act], nil
}

func newLiquidStakingDelta() *liquidStakingDelta {
	return &liquidStakingDelta{
		liquidStakingCacheManager: newLiquidStakingCache(),
		bucketTypeDeltaState:      make(map[uint64]deltaState),
		bucketInfoDeltaState:      make(map[uint64]deltaState),
	}
}

func (s *liquidStakingDelta) addBucketType(id uint64, bt *ContractStakingBucketType) error {
	if _, ok := s.bucketTypeDeltaState[id]; !ok {
		s.bucketTypeDeltaState[id] = deltaStateAdded
	} else {
		var err error
		s.bucketTypeDeltaState[id], err = s.bucketTypeDeltaState[id].transfer(deltaActionAdd)
		if err != nil {
			return err
		}
	}
	s.liquidStakingCacheManager.putBucketType(id, bt)
	return nil
}

func (s *liquidStakingDelta) updateBucketType(id uint64, bt *ContractStakingBucketType) error {
	if _, ok := s.bucketTypeDeltaState[id]; !ok {
		s.bucketTypeDeltaState[id] = deltaStateModified
	} else {
		var err error
		s.bucketTypeDeltaState[id], err = s.bucketTypeDeltaState[id].transfer(deltaActionModify)
		if err != nil {
			return err
		}
	}
	s.liquidStakingCacheManager.putBucketType(id, bt)
	return nil
}

func (s *liquidStakingDelta) addBucketInfo(id uint64, bi *ContractStakingBucketInfo) error {
	var err error
	if _, ok := s.bucketInfoDeltaState[id]; !ok {
		s.bucketInfoDeltaState[id] = deltaStateAdded
	} else {
		s.bucketInfoDeltaState[id], err = s.bucketInfoDeltaState[id].transfer(deltaActionAdd)
		if err != nil {
			return err
		}
	}
	s.liquidStakingCacheManager.putBucketInfo(id, bi)
	return nil
}

func (s *liquidStakingDelta) updateBucketInfo(id uint64, bi *ContractStakingBucketInfo) error {
	if _, ok := s.bucketInfoDeltaState[id]; !ok {
		s.bucketInfoDeltaState[id] = deltaStateModified
	} else {
		var err error
		s.bucketInfoDeltaState[id], err = s.bucketInfoDeltaState[id].transfer(deltaActionModify)
		if err != nil {
			return err
		}
	}
	s.liquidStakingCacheManager.putBucketInfo(id, bi)
	return nil
}

func (s *liquidStakingDelta) deleteBucketInfo(id uint64) error {
	if _, ok := s.bucketInfoDeltaState[id]; !ok {
		s.bucketInfoDeltaState[id] = deltaStateRemoved
	} else {
		var err error
		s.bucketInfoDeltaState[id], err = s.bucketInfoDeltaState[id].transfer(deltaActionRemove)
		if err != nil {
			return err
		}
	}
	s.liquidStakingCacheManager.deleteBucketInfo(id)
	return nil
}

func (s *liquidStakingDelta) addedBucketCnt() uint64 {
	addedBucketCnt := uint64(0)
	for _, state := range s.bucketInfoDeltaState {
		if state == deltaStateAdded {
			addedBucketCnt++
		}
	}
	return addedBucketCnt
}

func (s *liquidStakingDelta) addedBucketTypeCnt() uint64 {
	cnt := uint64(0)
	for _, state := range s.bucketTypeDeltaState {
		if state == deltaStateAdded {
			cnt++
		}
	}
	return cnt
}

func (s *liquidStakingDelta) isBucketDeleted(id uint64) bool {
	if _, ok := s.bucketInfoDeltaState[id]; ok {
		return s.bucketInfoDeltaState[id] == deltaStateRemoved
	}
	return false
}
