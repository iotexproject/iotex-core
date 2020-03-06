// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import "github.com/iotexproject/iotex-address/address"

type (
	// CandidateCenter is a struct to manage the candidates
	CandidateCenter struct {
		nameMap          map[string]*Candidate
		ownerMap         map[string]*Candidate
		operatorMap      map[string]*Candidate
		selfStkBucketMap map[uint64]*Candidate
	}
)

// NewCandidateCenter creates an instance of CandidateCenter
func NewCandidateCenter() *CandidateCenter {
	return &CandidateCenter{
		nameMap:          make(map[string]*Candidate),
		ownerMap:         make(map[string]*Candidate),
		operatorMap:      make(map[string]*Candidate),
		selfStkBucketMap: make(map[uint64]*Candidate),
	}
}

// Size returns number of candidates
func (m CandidateCenter) Size() int {
	return len(m.nameMap)
}

// ContainsName returns true if the map contains the candidate by name
func (m CandidateCenter) ContainsName(name string) bool {
	_, ok := m.nameMap[name]
	return ok
}

// ContainsOwner returns true if the map contains the candidate by owner
func (m CandidateCenter) ContainsOwner(owner address.Address) bool {
	_, ok := m.ownerMap[owner.String()]
	return ok
}

// ContainsOperator returns true if the map contains the candidate by operator
func (m CandidateCenter) ContainsOperator(operator address.Address) bool {
	_, ok := m.operatorMap[operator.String()]
	return ok
}

// ContainsSelfStakingBucket returns true if the map contains the self staking bucket index
func (m CandidateCenter) ContainsSelfStakingBucket(index uint64) bool {
	_, ok := m.selfStkBucketMap[index]
	return ok
}

// GetByName returns the candidate by name
func (m CandidateCenter) GetByName(name string) *Candidate {
	if d, ok := m.nameMap[name]; ok {
		return d.Clone()
	}
	return nil
}

// GetByOwner returns the candidate by owner
func (m CandidateCenter) GetByOwner(owner address.Address) *Candidate {
	if d, ok := m.ownerMap[owner.String()]; ok {
		return d.Clone()
	}
	return nil
}

// GetBySelfStakingIndex returns the candidate by self-staking index
func (m CandidateCenter) GetBySelfStakingIndex(index uint64) *Candidate {
	if d, ok := m.selfStkBucketMap[index]; ok {
		return d.Clone()
	}
	return nil
}

// Upsert adds a candidate into map, overwrites if already exist
func (m CandidateCenter) Upsert(d *Candidate) error {
	if err := m.checkCollision(d); err != nil {
		return err
	}

	if c, ok := m.ownerMap[d.Owner.String()]; ok {
		delete(m.nameMap, c.Name)
		delete(m.operatorMap, c.Operator.String())
		delete(m.selfStkBucketMap, c.SelfStakeBucketIdx)
	}

	m.nameMap[d.Name] = d
	m.ownerMap[d.Owner.String()] = d
	m.operatorMap[d.Operator.String()] = d
	m.selfStkBucketMap[d.SelfStakeBucketIdx] = d
	return nil
}

// Delete deletes the candidate by name
func (m CandidateCenter) Delete(owner address.Address) {
	d, ok := m.ownerMap[owner.String()]
	if !ok {
		return
	}

	delete(m.nameMap, d.Name)
	delete(m.ownerMap, d.Owner.String())
	delete(m.operatorMap, d.Operator.String())
	delete(m.selfStkBucketMap, d.SelfStakeBucketIdx)
}

func (m CandidateCenter) checkCollision(d *Candidate) error {
	if c, ok := m.nameMap[d.Name]; ok {
		if c.Owner.String() != d.Owner.String() {
			return ErrInvalidCanName
		}
	}

	if c, ok := m.operatorMap[d.Operator.String()]; ok {
		if c.Owner.String() != d.Owner.String() {
			return ErrInvalidOperator
		}
	}

	if c, ok := m.selfStkBucketMap[d.SelfStakeBucketIdx]; ok {
		if c.Owner.String() != d.Owner.String() {
			return ErrInvalidSelfStkIndex
		}
	}
	return nil
}
