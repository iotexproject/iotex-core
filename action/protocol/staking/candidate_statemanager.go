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

// const
const (
	_stakingCandCenter = "candCenter"
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
	return csm.StateManager.Load(_protocolID, _stakingCandCenter, &delta)
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
	return csm.WriteView(_protocolID, csm.DirtyView())
}
