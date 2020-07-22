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
)

type (
	// CandidateStateManager is candidate state manager on top of StateManager
	CandidateStateManager interface {
		protocol.StateManager
		CandCenter() CandidateCenter
		BucketPool() *BucketPool
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
	// TODO: we can store csm in a local cache, just as how statedb store the workingset
	// b/c most time the sm is used before, no need to create another clone
	csr, err := ConstructBaseView(sm)
	if err != nil {
		return nil, err
	}

	// make a copy of candidate center and bucket pool, so they can be modified by csm
	// and won't affect base view until being committed
	csm := &candSM{
		StateManager: sm,
		// TODO: remove CandidateCenter interface, no need for (*candCenter)
		candCenter: csr.CandCenter().Base().(*candCenter),
		bucketPool: csr.BucketPool().Clone(),
	}

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
	return csm.WriteView(protocolID, ConvertToViewData(csm))
}
