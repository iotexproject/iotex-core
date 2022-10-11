// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"sync"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action/protocol"
)

type (
	// candChange captures the change to candidates
	candChange struct {
		dirty map[string]*Candidate
	}

	// candBase is the confirmed base state
	candBase struct {
		lock             sync.RWMutex
		nameMap          map[string]*Candidate
		ownerMap         map[string]*Candidate
		operatorMap      map[string]*Candidate
		selfStkBucketMap map[uint64]*Candidate
	}

	// CandidateCenter is a struct to manage the candidates
	CandidateCenter struct {
		base   *candBase
		size   int
		change *candChange
	}
)

// listToCandChange creates a candChange from list
func listToCandChange(l CandidateList) (*candChange, error) {
	cv := newCandChange()

	for _, d := range l {
		if err := d.Validate(); err != nil {
			return nil, err
		}
		cv.dirty[d.Owner.String()] = d
	}
	return cv, nil
}

// NewCandidateCenter creates an instance of CandidateCenter
func NewCandidateCenter(all CandidateList) (*CandidateCenter, error) {
	delta, err := listToCandChange(all)
	if err != nil {
		return nil, err
	}

	c := CandidateCenter{
		base:   newCandBase(),
		change: delta,
	}

	if len(all) == 0 {
		return &c, nil
	}

	if err := c.Commit(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Size returns number of candidates
func (m *CandidateCenter) Size() int {
	return m.size
}

// All returns all candidates in candidate center
func (m *CandidateCenter) All() CandidateList {
	list := m.change.all()
	if list == nil {
		return m.base.all()
	}

	for _, d := range m.base.all() {
		if !m.change.containsOwner(d.Owner) {
			list = append(list, d.Clone())
		}
	}
	return list
}

// Base returns the confirmed base state
func (m CandidateCenter) Base() *CandidateCenter {
	return &CandidateCenter{
		base:   m.base,
		size:   len(m.base.ownerMap),
		change: newCandChange(),
	}
}

// Delta exports the pending changes
func (m *CandidateCenter) Delta() CandidateList {
	return m.change.all()
}

// SetDelta sets the delta
func (m *CandidateCenter) SetDelta(l CandidateList) error {
	if len(l) == 0 {
		m.change = nil
		m.change = newCandChange()
		m.size = m.base.size()
		return nil
	}

	var err error
	m.change, err = listToCandChange(l)
	if err != nil {
		return err
	}
	if m.base.size() == 0 {
		m.size = m.change.size()
		return nil
	}

	overlap := 0
	for _, v := range m.base.all() {
		if m.change.containsOwner(v.Owner) {
			overlap++
		}
	}
	m.size = m.base.size() + m.change.size() - overlap
	return nil
}

// Commit writes the change into base
func (m *CandidateCenter) Commit() error {
	size, err := m.base.Commit(m.change)
	if err != nil {
		return err
	}
	m.size = size
	m.change = nil
	m.change = newCandChange()
	return nil
}

// Sync syncs the data from state manager
func (m *CandidateCenter) Sync(sm protocol.StateManager) error {
	delta := CandidateList{}
	if err := sm.Unload(_protocolID, _stakingCandCenter, &delta); err != nil && err != protocol.ErrNoName {
		return err
	}

	// apply delta to the center
	return m.SetDelta(delta)
}

// ContainsName returns true if the map contains the candidate by name
func (m *CandidateCenter) ContainsName(name string) bool {
	if hit := m.change.containsName(name); hit {
		return true
	}

	if d, hit := m.base.getByName(name); hit {
		return !m.change.containsOwner(d.Owner)
	}
	return false
}

// ContainsOwner returns true if the map contains the candidate by owner
func (m *CandidateCenter) ContainsOwner(owner address.Address) bool {
	if owner == nil {
		return false
	}

	if hit := m.change.containsOwner(owner); hit {
		return true
	}

	_, hit := m.base.getByOwner(owner.String())
	return hit
}

// ContainsOperator returns true if the map contains the candidate by operator
func (m *CandidateCenter) ContainsOperator(operator address.Address) bool {
	if operator == nil {
		return false
	}

	if hit := m.change.containsOperator(operator); hit {
		return true
	}

	if d, hit := m.base.getByOperator(operator.String()); hit {
		return !m.change.containsOwner(d.Owner)
	}
	return false
}

// ContainsSelfStakingBucket returns true if the map contains the self staking bucket index
func (m *CandidateCenter) ContainsSelfStakingBucket(index uint64) bool {
	if hit := m.change.containsSelfStakingBucket(index); hit {
		return true
	}

	if d, hit := m.base.getBySelfStakingIndex(index); hit {
		return !m.change.containsOwner(d.Owner)
	}
	return false
}

// GetByName returns the candidate by name
func (m *CandidateCenter) GetByName(name string) *Candidate {
	if d := m.change.getByName(name); d != nil {
		return d
	}

	if d, hit := m.base.getByName(name); hit && !m.change.containsOwner(d.Owner) {
		return d.Clone()
	}
	return nil
}

// GetByOwner returns the candidate by owner
func (m *CandidateCenter) GetByOwner(owner address.Address) *Candidate {
	if owner == nil {
		return nil
	}

	if d := m.change.getByOwner(owner); d != nil {
		return d
	}

	if d, hit := m.base.getByOwner(owner.String()); hit {
		return d.Clone()
	}
	return nil
}

// GetBySelfStakingIndex returns the candidate by self-staking index
func (m *CandidateCenter) GetBySelfStakingIndex(index uint64) *Candidate {
	if d := m.change.getBySelfStakingIndex(index); d != nil {
		return d
	}

	if d, hit := m.base.getBySelfStakingIndex(index); hit && !m.change.containsOwner(d.Owner) {
		return d.Clone()
	}
	return nil
}

// Upsert adds a candidate into map, overwrites if already exist
func (m *CandidateCenter) Upsert(d *Candidate) error {
	if err := d.Validate(); err != nil {
		return err
	}

	if err := m.collision(d); err != nil {
		return err
	}

	if err := m.change.upsert(d); err != nil {
		return err
	}

	if _, hit := m.base.getByOwner(d.Owner.String()); !hit {
		m.size++
	}
	return nil
}

func (m *CandidateCenter) collision(d *Candidate) error {
	if err := m.change.collision(d); err != nil {
		return err
	}

	name, oper, self := m.base.collision(d)
	if name != nil && !m.change.containsOwner(name) {
		return ErrInvalidCanName
	}

	if oper != nil && !m.change.containsOwner(oper) {
		return ErrInvalidOperator
	}

	if self != nil && !m.change.containsOwner(self) {
		return ErrInvalidSelfStkIndex
	}
	return nil
}

//======================================
// candChange funcs
//======================================

func newCandChange() *candChange {
	return &candChange{
		dirty: make(map[string]*Candidate),
	}
}

func (cc *candChange) size() int {
	return len(cc.dirty)
}

func (cc *candChange) all() CandidateList {
	if len(cc.dirty) == 0 {
		return nil
	}

	list := make(CandidateList, 0, len(cc.dirty))
	for _, d := range cc.dirty {
		list = append(list, d.Clone())
	}
	return list
}

func (cc *candChange) containsName(name string) bool {
	for _, d := range cc.dirty {
		if name == d.Name {
			return true
		}
	}
	return false
}

func (cc *candChange) containsOwner(owner address.Address) bool {
	if owner == nil {
		return false
	}
	_, ok := cc.dirty[owner.String()]
	return ok
}

func (cc *candChange) containsOperator(operator address.Address) bool {
	for _, d := range cc.dirty {
		if address.Equal(operator, d.Operator) {
			return true
		}
	}
	return false
}

func (cc *candChange) containsSelfStakingBucket(index uint64) bool {
	for _, d := range cc.dirty {
		if index == d.SelfStakeBucketIdx {
			return true
		}
	}
	return false
}

func (cc *candChange) getByName(name string) *Candidate {
	for _, d := range cc.dirty {
		if name == d.Name {
			return d.Clone()
		}
	}
	return nil
}

func (cc *candChange) getByOwner(owner address.Address) *Candidate {
	if owner == nil {
		return nil
	}

	if d, ok := cc.dirty[owner.String()]; ok {
		return d.Clone()
	}
	return nil
}

func (cc *candChange) getBySelfStakingIndex(index uint64) *Candidate {
	for _, d := range cc.dirty {
		if index == d.SelfStakeBucketIdx {
			return d.Clone()
		}
	}
	return nil
}

func (cc *candChange) upsert(d *Candidate) error {
	if err := d.Validate(); err != nil {
		return err
	}
	cc.dirty[d.Owner.String()] = d
	return nil
}

func (cc *candChange) collision(d *Candidate) error {
	for _, c := range cc.dirty {
		if err := d.Collision(c); err != nil {
			return err
		}
	}
	return nil
}

func (cc *candChange) delete(owner address.Address) {
	if owner == nil {
		return
	}
	delete(cc.dirty, owner.String())
}

//======================================
// candBase funcs
//======================================

func newCandBase() *candBase {
	return &candBase{
		nameMap:          make(map[string]*Candidate),
		ownerMap:         make(map[string]*Candidate),
		operatorMap:      make(map[string]*Candidate),
		selfStkBucketMap: make(map[uint64]*Candidate),
	}
}

func (cb *candBase) size() int {
	cb.lock.RLock()
	defer cb.lock.RUnlock()
	return len(cb.ownerMap)
}

func (cb *candBase) all() CandidateList {
	cb.lock.RLock()
	defer cb.lock.RUnlock()
	if len(cb.ownerMap) == 0 {
		return nil
	}

	list := make(CandidateList, 0, len(cb.ownerMap))
	for _, d := range cb.ownerMap {
		list = append(list, d.Clone())
	}
	return list
}

func (cb *candBase) Clone() *candBase {
	cb.lock.RLock()
	defer cb.lock.RUnlock()
	clone := newCandBase()
	for name, cand := range cb.nameMap {
		clone.nameMap[name] = cand.Clone()
	}
	for owner, cand := range cb.ownerMap {
		clone.ownerMap[owner] = cand.Clone()
	}
	for operator, cand := range cb.operatorMap {
		clone.operatorMap[operator] = cand.Clone()
	}
	for bucket, cand := range cb.selfStkBucketMap {
		clone.selfStkBucketMap[bucket] = cand.Clone()
	}

	return clone
}

func (cb *candBase) Commit(change *candChange) (int, error) {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	for _, v := range change.dirty {
		if err := v.Validate(); err != nil {
			return 0, err
		}
		d := v.Clone()
		cb.ownerMap[d.Owner.String()] = d
		cb.nameMap[d.Name] = d
		cb.operatorMap[d.Operator.String()] = d
		cb.selfStkBucketMap[d.SelfStakeBucketIdx] = d
	}
	return len(cb.ownerMap), nil
}

func (cb *candBase) getByName(name string) (*Candidate, bool) {
	cb.lock.RLock()
	defer cb.lock.RUnlock()
	d, ok := cb.nameMap[name]
	return d, ok
}

func (cb *candBase) getByOwner(name string) (*Candidate, bool) {
	cb.lock.RLock()
	defer cb.lock.RUnlock()
	d, ok := cb.ownerMap[name]
	return d, ok
}

func (cb *candBase) getByOperator(name string) (*Candidate, bool) {
	cb.lock.RLock()
	defer cb.lock.RUnlock()
	d, ok := cb.operatorMap[name]
	return d, ok
}

func (cb *candBase) getBySelfStakingIndex(index uint64) (*Candidate, bool) {
	cb.lock.RLock()
	defer cb.lock.RUnlock()
	d, ok := cb.selfStkBucketMap[index]
	return d, ok
}

func (cb *candBase) collision(d *Candidate) (address.Address, address.Address, address.Address) {
	cb.lock.RLock()
	defer cb.lock.RUnlock()
	var name, oper, self address.Address
	if c, hit := cb.nameMap[d.Name]; hit && !address.Equal(c.Owner, d.Owner) {
		name = c.Owner
	}

	if c, hit := cb.operatorMap[d.Operator.String()]; hit && !address.Equal(c.Owner, d.Owner) {
		oper = c.Owner
	}

	if c, hit := cb.selfStkBucketMap[d.SelfStakeBucketIdx]; hit && !address.Equal(c.Owner, d.Owner) {
		self = c.Owner
	}
	return name, oper, self
}

func (cb *candBase) delete(owner address.Address) {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	if d, hit := cb.ownerMap[owner.String()]; hit {
		delete(cb.nameMap, d.Name)
		delete(cb.ownerMap, d.Owner.String())
		delete(cb.operatorMap, d.Operator.String())
		delete(cb.selfStkBucketMap, d.SelfStakeBucketIdx)
	}
}
