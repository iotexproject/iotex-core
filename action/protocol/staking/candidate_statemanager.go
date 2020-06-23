// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state"
)

type (
	// CandidateStateManager is candidate manager on top of StateMangaer
	CandidateStateManager interface {
		protocol.StateManager
		// candidate-related
		Size() int
		ContainsName(string) bool
		ContainsOwner(address.Address) bool
		ContainsOperator(address.Address) bool
		ContainsSelfStakingBucket(uint64) bool
		GetByName(string) *Candidate
		GetByOwner(address.Address) *Candidate
		GetBySelfStakingIndex(uint64) *Candidate
		Upsert(*Candidate) error
		Commit() error

		// private functions
		getBucket(index uint64) (*VoteBucket, error)
		getAllBuckets() ([]*VoteBucket, error)
		getBucketsWithIndices(indices BucketIndices) ([]*VoteBucket, error)
		updateBucket(index uint64, bucket *VoteBucket) error
		putBucket(bucket *VoteBucket) (uint64, error)
		delBucket(index uint64) error
		putBucketAndIndex(bucket *VoteBucket) (uint64, error)
		delBucketAndIndex(owner, cand address.Address, index uint64) error

		putVoterBucketIndex(addr address.Address, index uint64) error
		delVoterBucketIndex(addr address.Address, index uint64) error
		putCandBucketIndex(addr address.Address, index uint64) error
		delCandBucketIndex(addr address.Address, index uint64) error
	}

	candSM struct {
		protocol.StateManager
		CandidateCenter
	}
)

// NewCandidateStateManager returns a new CandidateStateManager instance
func NewCandidateStateManager(sm protocol.StateManager, c CandidateCenter) (CandidateStateManager, error) {
	if sm == nil {
		return nil, ErrMissingField
	}

	csm := candSM{
		sm,
		c,
	}

	// extract view change from SM
	delta, err := retrieveDeltaFromSM(sm)
	switch errors.Cause(err) {
	case ErrTypeAssertion:
		{
			return nil, errors.Wrap(err, "failed to create CandidateStateManager")
		}
	case ErrNilParameters:
		{
			return &csm, nil
		}
	}

	// add delta to the center
	if err := c.SetDelta(delta); err != nil {
		return nil, err
	}
	return &csm, nil
}

// Upsert writes the candidate into state manager and cand center
func (csm *candSM) Upsert(d *Candidate) error {
	if err := csm.CandidateCenter.Upsert(d); err != nil {
		return err
	}

	if err := putCandidate(csm.StateManager, d); err != nil {
		return err
	}

	delta := csm.Delta()
	if len(delta) == 0 {
		return nil
	}

	ser, err := delta.Serialize()
	if err != nil {
		return err
	}

	// load change to sm
	csm.StateManager.Load(protocolID, ser)
	return nil
}

func (csm *candSM) Commit() error {
	if err := csm.CandidateCenter.Commit(); err != nil {
		return err
	}

	// write update view back to state factory
	return csm.WriteView(protocolID, csm.CandidateCenter)
}

func retrieveDeltaFromSM(sm protocol.StateManager) (CandidateList, error) {
	v, err := sm.Unload(protocolID)
	if err != nil {
		if errors.Cause(err) == protocol.ErrNoName {
			// the protocol hasn't pushed any data yet, return empty
			return nil, ErrNilParameters
		}
		return nil, err
	}

	ser, ok := v.([]byte)
	if !ok {
		return nil, errors.Wrap(ErrTypeAssertion, "failed to retrieveDeltaFromSM, expecting []byte")
	}

	l := CandidateList{}
	if err := l.Deserialize(ser); err != nil {
		return nil, errors.Wrap(err, "failed to retrieveDeltaFromSM")
	}
	return l, nil
}

func (csm *candSM) getBucketsWithIndices(indices BucketIndices) ([]*VoteBucket, error) {
	buckets := make([]*VoteBucket, 0, len(indices))
	for _, i := range indices {
		b, err := csm.getBucket(i)
		if err != nil && err != ErrWithdrawnBucket {
			return buckets, err
		}
		buckets = append(buckets, b)
	}
	return buckets, nil
}

func (csm *candSM) getBucket(index uint64) (*VoteBucket, error) {
	var vb VoteBucket
	var err error
	if _, err = csm.State(&vb,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(bucketKey(index))); err != nil {
		return nil, err
	}
	var tc totalBucketCount
	if _, err := csm.State(
		&tc,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(TotalBucketKey)); err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return nil, err
	}
	if errors.Cause(err) == state.ErrStateNotExist && index < tc.Count() {
		return nil, ErrWithdrawnBucket
	}
	return &vb, nil
}

func (csm *candSM) updateBucket(index uint64, bucket *VoteBucket) error {
	if _, err := csm.getBucket(index); err != nil {
		return err
	}

	_, err := csm.PutState(
		bucket,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(bucketKey(index)))
	return err
}

func (csm *candSM) putBucket(bucket *VoteBucket) (uint64, error) {
	var tc totalBucketCount
	if _, err := csm.State(
		&tc,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(TotalBucketKey)); err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return 0, err
	}

	index := tc.Count()
	// Add index inside bucket
	bucket.Index = index
	if _, err := csm.PutState(
		bucket,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(bucketKey(index))); err != nil {
		return 0, err
	}
	tc.count++
	_, err := csm.PutState(
		&tc,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(TotalBucketKey))
	return index, err
}

func (csm *candSM) delBucket(index uint64) error {
	_, err := csm.DelState(
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(bucketKey(index)))
	return err
}

func (csm *candSM) getAllBuckets() ([]*VoteBucket, error) {
	// bucketKey is prefixed with const bucket = '0', all bucketKey will compare less than []byte{bucket+1}
	maxKey := []byte{_bucket + 1}
	_, iter, err := csm.States(
		protocol.NamespaceOption(StakingNameSpace),
		protocol.FilterOption(func(k, v []byte) bool {
			return bytes.HasPrefix(k, []byte{_bucket})
		}, bucketKey(0), maxKey))
	if errors.Cause(err) == state.ErrStateNotExist {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	buckets := make([]*VoteBucket, 0, iter.Size())
	for i := 0; i < iter.Size(); i++ {
		vb := &VoteBucket{}
		if err := iter.Next(vb); err != nil {
			return nil, errors.Wrapf(err, "failed to deserialize bucket")
		}
		buckets = append(buckets, vb)
	}
	return buckets, nil
}
