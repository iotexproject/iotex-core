// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state"
)

// const
const (
	stakingCandCenter = "candCenter"
)

type (
	// CandidateStateManager is candidate state manager on top of StateManager
	CandidateStateManager interface {
		protocol.StateManager
		// candidate and bucket pool related
		DirtyView() *ViewData
		ContainsName(string) bool
		ContainsOwner(address.Address) bool
		ContainsOperator(address.Address) bool
		ContainsSelfStakingBucket(uint64) bool
		GetByName(string) *Candidate
		GetByOwner(address.Address) *Candidate
		GetBySelfStakingIndex(uint64) *Candidate
		Upsert(*Candidate) error
		CreditBucketPool(*big.Int) error
		DebitBucketPool(*big.Int, bool) error
		Commit() error
	}

	candSM struct {
		protocol.StateManager
		candCenter *CandidateCenter
		bucketPool *BucketPool
	}
)

// NewCandidateStateManager returns a new CandidateStateManager instance
func NewCandidateStateManager(sm protocol.StateManager, enableSMStorage bool) (CandidateStateManager, error) {
	// TODO: we can store csm in a local cache, just as how statedb store the workingset
	// b/c most time the sm is used before, no need to create another clone
	csr, err := ConstructBaseView(sm)
	if err != nil {
		return nil, err
	}

	// make a copy of candidate center and bucket pool, so they can be modified by csm
	// and won't affect base view until being committed
	view := csr.BaseView()
	csm := &candSM{
		StateManager: sm,
		candCenter:   view.candCenter.Base(),
		bucketPool:   view.bucketPool.Copy(enableSMStorage),
	}

	// extract view change from SM
	if err := csm.bucketPool.Sync(sm); err != nil {
		return nil, errors.Wrap(err, "failed to sync bucket pool")
	}

	if err := csm.candCenter.Sync(sm); err != nil {
		return nil, errors.Wrap(err, "failed to sync candidate center")
	}
	return csm, nil
}

func newEmptyCsm(sm protocol.StateManager) *candSM {
	return &candSM{
		StateManager: sm,
	}
}

// DirtyView is csm's current state, which reflects base view + applying delta saved in csm's dock
func (csm *candSM) DirtyView() *ViewData {
	return &ViewData{
		candCenter: csm.candCenter,
		bucketPool: csm.bucketPool,
	}
}

func (csm *candSM) ContainsName(name string) bool {
	return csm.candCenter.ContainsName(name)
}

func (csm *candSM) ContainsOwner(addr address.Address) bool {
	return csm.candCenter.ContainsOwner(addr)
}

func (csm *candSM) ContainsOperator(addr address.Address) bool {
	return csm.candCenter.ContainsOperator(addr)
}

func (csm *candSM) ContainsSelfStakingBucket(index uint64) bool {
	return csm.candCenter.ContainsSelfStakingBucket(index)
}

func (csm *candSM) GetByName(name string) *Candidate {
	return csm.candCenter.GetByName(name)
}

func (csm *candSM) GetByOwner(addr address.Address) *Candidate {
	return csm.candCenter.GetByOwner(addr)
}

func (csm *candSM) GetBySelfStakingIndex(index uint64) *Candidate {
	return csm.candCenter.GetBySelfStakingIndex(index)
}

// Upsert writes the candidate into state manager and cand center
func (csm *candSM) Upsert(d *Candidate) error {
	if err := csm.candCenter.Upsert(d); err != nil {
		return err
	}

	if err := putCandidate(csm.StateManager, d); err != nil {
		return err
	}

	delta := csm.candCenter.Delta()
	if len(delta) == 0 {
		return nil
	}

	// load change to sm
	return csm.StateManager.Load(protocolID, stakingCandCenter, &delta)
}

func (csm *candSM) CreditBucketPool(amount *big.Int) error {
	return csm.bucketPool.CreditPool(csm.StateManager, amount)
}

func (csm *candSM) DebitBucketPool(amount *big.Int, newBucket bool) error {
	return csm.bucketPool.DebitPool(csm.StateManager, amount, newBucket)
}

func (csm *candSM) Commit() error {
	if err := csm.candCenter.Commit(); err != nil {
		return err
	}

	if err := csm.bucketPool.Commit(csm.StateManager); err != nil {
		return err
	}

	// write updated view back to state factory
	return csm.WriteView(protocolID, csm.DirtyView())
}

func (csm *candSM) updateBucket(index uint64, bucket *VoteBucket) error {
	if _, err := newEmptyCsr(csm).getBucket(index); err != nil {
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

func (csm *candSM) putBucketAndIndex(bucket *VoteBucket) (uint64, error) {
	index, err := csm.putBucket(bucket)
	if err != nil {
		return 0, errors.Wrap(err, "failed to put bucket")
	}

	if err := putVoterBucketIndex(csm, bucket.Owner, index); err != nil {
		return 0, errors.Wrap(err, "failed to put bucket index")
	}

	if err := putCandBucketIndex(csm, bucket.Candidate, index); err != nil {
		return 0, errors.Wrap(err, "failed to put candidate index")
	}
	return index, nil
}

func (csm *candSM) delBucketAndIndex(owner, cand address.Address, index uint64) error {
	if err := csm.delBucket(index); err != nil {
		return errors.Wrap(err, "failed to delete bucket")
	}

	if err := delVoterBucketIndex(csm, owner, index); err != nil {
		return errors.Wrap(err, "failed to delete bucket index")
	}

	if err := delCandBucketIndex(csm, cand, index); err != nil {
		return errors.Wrap(err, "failed to delete candidate index")
	}
	return nil
}

// <<<<<<< HEAD
// func getTotalBucketCount(sr protocol.StateReader) (uint64, error) {
// 	var tc totalBucketCount
// 	_, err := sr.State(
// 		&tc,
// 		protocol.NamespaceOption(StakingNameSpace),
// 		protocol.KeyOption(TotalBucketKey))
// 	return tc.count, err
// }

// func getBucket(sr protocol.StateReader, index uint64) (*VoteBucket, error) {
// 	var vb VoteBucket
// 	var err error
// 	if _, err = sr.State(
// 		&vb,
// 		protocol.NamespaceOption(StakingNameSpace),
// 		protocol.KeyOption(bucketKey(index))); err != nil {
// 		return nil, err
// 	}
// 	var tc totalBucketCount
// 	if _, err := sr.State(
// 		&tc,
// 		protocol.NamespaceOption(StakingNameSpace),
// 		protocol.KeyOption(TotalBucketKey)); err != nil && errors.Cause(err) != state.ErrStateNotExist {
// 		return nil, err
// 	}
// 	if errors.Cause(err) == state.ErrStateNotExist && index < tc.Count() {
// 		return nil, ErrWithdrawnBucket
// 	}
// 	return &vb, nil
// }

// func updateBucket(sm protocol.StateManager, index uint64, bucket *VoteBucket) error {
// 	if _, err := getBucket(sm, index); err != nil {
// 		return err
// 	}

// 	_, err := sm.PutState(
// 		bucket,
// 		protocol.NamespaceOption(StakingNameSpace),
// 		protocol.KeyOption(bucketKey(index)))
// 	return err
// }

// func putBucket(sm protocol.StateManager, bucket *VoteBucket) (uint64, error) {
// 	var tc totalBucketCount
// 	if _, err := sm.State(
// 		&tc,
// 		protocol.NamespaceOption(StakingNameSpace),
// 		protocol.KeyOption(TotalBucketKey)); err != nil && errors.Cause(err) != state.ErrStateNotExist {
// 		return 0, err
// 	}

// 	index := tc.Count()
// 	// Add index inside bucket
// 	bucket.Index = index
// 	if _, err := sm.PutState(
// 		bucket,
// 		protocol.NamespaceOption(StakingNameSpace),
// 		protocol.KeyOption(bucketKey(index))); err != nil {
// 		return 0, err
// 	}
// 	tc.count++
// 	_, err := sm.PutState(
// 		&tc,
// 		protocol.NamespaceOption(StakingNameSpace),
// 		protocol.KeyOption(TotalBucketKey))
// 	return index, err
// }

// func delBucket(sm protocol.StateManager, index uint64) error {
// 	_, err := sm.DelState(
// 		protocol.NamespaceOption(StakingNameSpace),
// 		protocol.KeyOption(bucketKey(index)))
// 	return err
// }

// func getAllBuckets(sr protocol.StateReader) ([]*VoteBucket, uint64, error) {
// 	// bucketKey is prefixed with const bucket = '0', all bucketKey will compare less than []byte{bucket+1}
// 	maxKey := []byte{_bucket + 1}
// 	height, iter, err := sr.States(
// 		protocol.NamespaceOption(StakingNameSpace),
// 		protocol.FilterOption(func(k, v []byte) bool {
// 			return bytes.HasPrefix(k, []byte{_bucket})
// 		}, bucketKey(0), maxKey))
// 	if err != nil {
// 		return nil, height, err
// 	}

// 	buckets := make([]*VoteBucket, 0, iter.Size())
// 	for i := 0; i < iter.Size(); i++ {
// 		vb := &VoteBucket{}
// 		if err := iter.Next(vb); err != nil {
// 			return nil, height, errors.Wrapf(err, "failed to deserialize bucket")
// 		}
// 		buckets = append(buckets, vb)
// 	}
// 	return buckets, height, nil
// }

// func getBucketsWithIndices(sr protocol.StateReader, indices BucketIndices) ([]*VoteBucket, error) {
// 	buckets := make([]*VoteBucket, 0, len(indices))
// 	for _, i := range indices {
// 		b, err := getBucket(sr, i)
// 		if err != nil && err != ErrWithdrawnBucket {
// 			return buckets, err
// 		}
// 		buckets = append(buckets, b)
// 	}
// 	return buckets, nil
// }

// =======
// >>>>>>> move static functions into CandidateStateManager or CandidateStateReader
