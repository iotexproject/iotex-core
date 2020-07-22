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

type (
	// candidateBucketCenter contains candidate center and bucket pool
	candidateBucketCenter interface {
		// TODO: remove CandidateCenter interface, return *candCenter
		CandCenter() CandidateCenter
		BucketPool() *BucketPool
	}

	// CandidateStateManager is candidate state manager on top of StateManager
	CandidateStateManager interface {
		protocol.StateManager
		// candidate and bucket pool related
		Size() int
		ContainsName(string) bool
		ContainsOwner(address.Address) bool
		ContainsOperator(address.Address) bool
		ContainsSelfStakingBucket(uint64) bool
		GetByName(string) *Candidate
		GetByOwner(address.Address) *Candidate
		GetBySelfStakingIndex(uint64) *Candidate
		Upsert(*Candidate) error
		CreditBucketPool(*big.Int, bool) error
		DebitBucketPool(*big.Int, bool, bool) error
		Commit() error
	}

	candSM struct {
		protocol.StateManager
		candCenter *candCenter
		bucketPool *BucketPool
	}
)

// NewCandidateStateManager returns a new CandidateStateManager instance
func NewCandidateStateManager(sm protocol.StateManager) (CandidateStateManager, error) {
	if sm == nil {
		return nil, ErrMissingField
	}

	v, err := sm.ReadView(protocolID)
	if err != nil {
		return nil, err
	}

	csm, ok := v.(*candSM)
	if !ok {
		return nil, errors.Wrap(protocol.ErrTypeAssertion, "expecting *candSM")
	}

	// TODO: remove CandidateCenter interface, no need for (*candCenter)
	csm.StateManager = sm
	csm.candCenter = csm.candCenter.Base().(*candCenter)
	csm.bucketPool = csm.bucketPool.Clone()

	// extract view change from SM
	if err := csm.bucketPool.SyncPool(sm); err != nil {
		return nil, err
	}

	// TODO: remove CandidateCenter interface, convert the code below to candCenter.SyncCenter()
	ser, err := protocol.UnloadAndAssertBytes(sm, protocolID)
	switch errors.Cause(err) {
	case protocol.ErrTypeAssertion:
		return nil, errors.Wrap(err, "failed to create CandidateStateManager")
	case protocol.ErrNoName:
		return csm, nil
	}

	delta := CandidateList{}
	if err := delta.Deserialize(ser); err != nil {
		return nil, err
	}

	// apply delta to the center
	if err := csm.candCenter.SetDelta(delta); err != nil {
		return nil, err
	}
	return csm, nil
}

func (csm *candSM) CandCenter() CandidateCenter {
	return csm.candCenter
}

func (csm *candSM) BucketPool() *BucketPool {
	return csm.bucketPool
}

func (csm *candSM) Size() int {
	return csm.candCenter.Size()
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

	ser, err := delta.Serialize()
	if err != nil {
		return err
	}

	// load change to sm
	csm.StateManager.Load(protocolID, ser)
	return nil
}

func (csm *candSM) CreditBucketPool(amount *big.Int, create bool) error {
	return csm.bucketPool.CreditPool(csm.StateManager, amount, create)
}

func (csm *candSM) DebitBucketPool(amount *big.Int, newBucket, create bool) error {
	return csm.bucketPool.DebitPool(csm.StateManager, amount, newBucket, create)
}

func (csm *candSM) Commit() error {
	if err := csm.candCenter.Commit(); err != nil {
		return err
	}

	if err := csm.bucketPool.Commit(csm.StateManager); err != nil {
		return err
	}

	// write update view back to state factory
	return csm.WriteView(protocolID, csm)
}

func getOrCreateCandCenter(sr protocol.StateReader) (candidateBucketCenter, error) {
	c, err := getCandCenter(sr)
	if err != nil {
		if errors.Cause(err) == protocol.ErrNoName {
			// the view does not exist yet, create it
			return createCandCenter(sr)
		}
		return nil, err
	}
	return c, nil
}

func getCandCenter(sr protocol.StateReader) (candidateBucketCenter, error) {
	v, err := sr.ReadView(protocolID)
	if err != nil {
		return nil, err
	}

	if center, ok := v.(candidateBucketCenter); ok {
		return center, nil
	}
	return nil, errors.Wrap(protocol.ErrTypeAssertion, "expecting candidateBucketCenter")
}

func createCandCenter(sr protocol.StateReader) (candidateBucketCenter, error) {
	all, err := loadCandidatesFromSR(sr)
	if err != nil {
		return nil, err
	}

	center, err := NewCandidateCenter(all)
	if err != nil {
		return nil, err
	}

	pool, err := NewBucketPool(sr)
	if err != nil {
		return nil, err
	}

	// TODO: remove CandidateCenter interface, no need for (*candCenter)
	return &candSM{
		StateManager: nil,
		candCenter:   center.(*candCenter),
		bucketPool:   pool,
	}, nil
}

func loadCandidatesFromSR(sr protocol.StateReader) (CandidateList, error) {
	_, iter, err := sr.States(protocol.NamespaceOption(CandidateNameSpace))
	if errors.Cause(err) == state.ErrStateNotExist {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	cands := make(CandidateList, 0, iter.Size())
	for i := 0; i < iter.Size(); i++ {
		c := &Candidate{}
		if err := iter.Next(c); err != nil {
			return nil, errors.Wrapf(err, "failed to deserialize candidate")
		}
		cands = append(cands, c)
	}
	return cands, nil
}
