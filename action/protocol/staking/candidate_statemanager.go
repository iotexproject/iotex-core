// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

type (
	// BucketSet related to setting bucket
	BucketSet interface {
		updateBucket(index uint64, bucket *VoteBucket) error
		putBucket(bucket *VoteBucket) (uint64, error)
		delBucket(index uint64) error
		putBucketAndIndex(bucket *VoteBucket) (uint64, error)
		delBucketAndIndex(owner, cand address.Address, index uint64) error
	}
	// CandidateSet related to setting candidates
	CandidateSet interface {
		putCandidate(*Candidate) error
		delCandidate(address.Address) error
		putVoterBucketIndex(address.Address, uint64) error
		delVoterBucketIndex(address.Address, uint64) error
		putCandBucketIndex(address.Address, uint64) error
		delCandBucketIndex(address.Address, uint64) error
	}
	// CandidateStateManager is candidate state manager on top of StateManager
	CandidateStateManager interface {
		BucketSet
		BucketGetByIndex
		CandidateSet
		// candidate and bucket pool related
		DirtyView() *ViewData
		ContainsName(string) bool
		ContainsOwner(address.Address) bool
		ContainsOperator(address.Address) bool
		ContainsSelfStakingBucket(uint64) bool
		GetByName(string) *Candidate
		GetByOwner(address.Address) *Candidate
		GetByIdentifier(address.Address) *Candidate
		Upsert(*Candidate) error
		CreditBucketPool(*big.Int) error
		DebitBucketPool(*big.Int, bool) error
		Commit(context.Context) error
		SM() protocol.StateManager
		SR() protocol.StateReader
	}

	// CandidiateStateCommon is the common interface for candidate state manager and reader
	CandidiateStateCommon interface {
		ContainsSelfStakingBucket(uint64) bool
		GetByIdentifier(address.Address) *Candidate
		SR() protocol.StateReader
		BucketGetByIndex
	}

	candSM struct {
		protocol.StateManager
		candCenter *CandidateCenter
		bucketPool *BucketPool
	}
)

// NewCandidateStateManager returns a new CandidateStateManager instance
func NewCandidateStateManager(sm protocol.StateManager) (CandidateStateManager, error) {
	// TODO: we can store csm in a local cache, just as how statedb store the workingset
	// b/c most time the sm is used before, no need to create another clone
	csr, err := ConstructBaseView(sm)
	if err != nil {
		return nil, err
	}

	// make a copy of candidate center and bucket pool, so they can be modified by csm
	// and won't affect base view until being committed
	view := csr.BaseView()
	return &candSM{
		StateManager: sm,
		candCenter:   view.candCenter,
		bucketPool:   view.bucketPool,
	}, nil
}

func newCandidateStateManager(sm protocol.StateManager) CandidateStateManager {
	return &candSM{
		StateManager: sm,
	}
}

func (csm *candSM) SM() protocol.StateManager {
	return csm.StateManager
}

func (csm *candSM) SR() protocol.StateReader {
	return csm.StateManager
}

// DirtyView is csm's current state, which reflects base view + applying delta saved in csm's dock
func (csm *candSM) DirtyView() *ViewData {
	v, err := csm.StateManager.ReadView(_protocolID)
	if err != nil {
		log.S().Panic("failed to read view", zap.Error(err))
	}
	return &ViewData{
		candCenter:     csm.candCenter,
		bucketPool:     csm.bucketPool,
		contractsStake: v.(*ViewData).contractsStake,
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

func (csm *candSM) GetByIdentifier(addr address.Address) *Candidate {
	return csm.candCenter.GetByIdentifier(addr)
}

// Upsert writes the candidate into state manager and cand center
func (csm *candSM) Upsert(d *Candidate) error {
	if err := csm.candCenter.Upsert(d); err != nil {
		return err
	}

	return csm.putCandidate(d)
}

func (csm *candSM) CreditBucketPool(amount *big.Int) error {
	return csm.bucketPool.CreditPool(csm.StateManager, amount)
}

func (csm *candSM) DebitBucketPool(amount *big.Int, newBucket bool) error {
	return csm.bucketPool.DebitPool(csm, amount, newBucket)
}

func (csm *candSM) Commit(ctx context.Context) error {
	view := csm.DirtyView()
	if err := view.Commit(ctx, csm); err != nil {
		return err
	}

	// write updated view back to state factory
	return csm.WriteView(_protocolID, view)
}

func (csm *candSM) getBucket(index uint64) (*VoteBucket, error) {
	return newCandidateStateReader(csm).getBucket(index)
}

func (csm *candSM) updateBucket(index uint64, bucket *VoteBucket) error {
	if _, err := csm.getBucket(index); err != nil {
		return err
	}

	_, err := csm.PutState(
		bucket,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(bucketKey(index)))
	return err
}

func (csm *candSM) putBucket(bucket *VoteBucket) (uint64, error) {
	var tc totalBucketCount
	if _, err := csm.State(
		&tc,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(TotalBucketKey)); err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return 0, err
	}

	index := tc.Count()
	// Add index inside bucket
	bucket.Index = index
	if _, err := csm.PutState(
		bucket,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(bucketKey(index))); err != nil {
		return 0, err
	}
	tc.count++
	_, err := csm.PutState(
		&tc,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(TotalBucketKey))
	return index, err
}

func (csm *candSM) delBucket(index uint64) error {
	_, err := csm.DelState(
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(bucketKey(index)))
	return err
}

func (csm *candSM) putBucketAndIndex(bucket *VoteBucket) (uint64, error) {
	index, err := csm.putBucket(bucket)
	if err != nil {
		return 0, errors.Wrap(err, "failed to put bucket")
	}

	if err := csm.putVoterBucketIndex(bucket.Owner, index); err != nil {
		return 0, errors.Wrap(err, "failed to put bucket index")
	}

	if err := csm.putCandBucketIndex(bucket.Candidate, index); err != nil {
		return 0, errors.Wrap(err, "failed to put candidate index")
	}
	return index, nil
}

func (csm *candSM) delBucketAndIndex(owner, cand address.Address, index uint64) error {
	if err := csm.delBucket(index); err != nil {
		return errors.Wrap(err, "failed to delete bucket")
	}

	if err := csm.delVoterBucketIndex(owner, index); err != nil {
		return errors.Wrap(err, "failed to delete bucket index")
	}

	if err := csm.delCandBucketIndex(cand, index); err != nil {
		return errors.Wrap(err, "failed to delete candidate index")
	}
	return nil
}

func (csm *candSM) putBucketIndex(addr address.Address, prefix byte, index uint64) error {
	var (
		bis BucketIndices
		key = AddrKeyWithPrefix(addr, prefix)
	)
	if _, err := csm.State(
		&bis,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(key)); err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return err
	}
	bis.addBucketIndex(index)
	_, err := csm.PutState(
		&bis,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(key))
	return err
}

func (csm *candSM) putVoterBucketIndex(addr address.Address, index uint64) error {
	return csm.putBucketIndex(addr, _voterIndex, index)
}

func (csm *candSM) delBucketIndex(addr address.Address, prefix byte, index uint64) error {
	var (
		bis BucketIndices
		key = AddrKeyWithPrefix(addr, prefix)
	)
	if _, err := csm.State(
		&bis,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(key)); err != nil {
		return err
	}
	bis.deleteBucketIndex(index)

	var err error
	if len(bis) == 0 {
		_, err = csm.DelState(
			protocol.NamespaceOption(_stakingNameSpace),
			protocol.KeyOption(key))
	} else {
		_, err = csm.PutState(
			&bis,
			protocol.NamespaceOption(_stakingNameSpace),
			protocol.KeyOption(key))
	}
	return err
}

func (csm *candSM) delVoterBucketIndex(addr address.Address, index uint64) error {
	return csm.delBucketIndex(addr, _voterIndex, index)
}

func (csm *candSM) putCandidate(d *Candidate) error {
	_, err := csm.PutState(d, protocol.NamespaceOption(_candidateNameSpace), protocol.KeyOption(d.GetIdentifier().Bytes()))
	return err
}

func (csm *candSM) putCandBucketIndex(addr address.Address, index uint64) error {
	return csm.putBucketIndex(addr, _candIndex, index)
}

func (csm *candSM) delCandidate(name address.Address) error {
	_, err := csm.DelState(protocol.NamespaceOption(_candidateNameSpace), protocol.KeyOption(name.Bytes()))
	return err
}

func (csm *candSM) delCandBucketIndex(addr address.Address, index uint64) error {
	return csm.delBucketIndex(addr, _candIndex, index)
}
