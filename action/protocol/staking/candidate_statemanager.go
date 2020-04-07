// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
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

func getOrCreateCandCenter(sr protocol.StateReader) (CandidateCenter, error) {
	c, err := getCandCenter(sr)
	if err != nil {
		if errors.Cause(err) == protocol.ErrNoName {
			// the view does not exist yet, create it
			cc, err := createCandCenter(sr)
			return cc, err
		}
		return nil, err
	}
	return c, nil
}

func getCandCenter(sr protocol.StateReader) (CandidateCenter, error) {
	v, err := sr.ReadView(protocolID)
	if err != nil {
		return nil, err
	}

	if center, ok := v.(CandidateCenter); ok {
		return center, nil
	}
	return nil, errors.Wrap(ErrTypeAssertion, "expecting CandidateCenter")
}

func createCandCenter(sr protocol.StateReader) (CandidateCenter, error) {
	all, err := loadCandidatesFromSR(sr)
	if err != nil {
		return nil, err
	}

	return NewCandidateCenter(all)
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
