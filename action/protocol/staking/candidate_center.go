// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"sync"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action"
)

type (
	// candChange captures the change to candidates
	candChange struct {
		candidates []*Candidate
		dirty      map[string]*Candidate
	}

	// candBase is the confirmed base state
	candBase struct {
		lock             sync.RWMutex
		nameMap          map[string]*Candidate
		ownerMap         map[string]*Candidate
		operatorMap      map[string]*Candidate
		identifierMap    map[string]*Candidate
		selfStkBucketMap map[uint64]*Candidate
		owners           CandidateList
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
		cv.candidates = append(cv.candidates, d)
		cv.dirty[d.GetIdentifier().String()] = d
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
	list := m.change.view()
	if list == nil {
		return m.base.all()
	}

	for _, d := range m.base.all() {
		if !m.change.containsIdentifier(d.GetIdentifier()) {
			list = append(list, d.Clone())
		}
	}
	return list
}

func (m *CandidateCenter) Clone() *CandidateCenter {
	return &CandidateCenter{
		base:   m.base.clone(),
		size:   m.size,
		change: m.change.clone(),
	}
}

// Base returns the confirmed base state
func (m CandidateCenter) Base() *CandidateCenter {
	return &CandidateCenter{
		base:   m.base,
		size:   len(m.base.ownerMap),
		change: newCandChange(),
	}
}

// Commit writes the change into base
func (m *CandidateCenter) Commit() error {
	size, err := m.base.commit(m.change, false)
	if err != nil {
		return err
	}
	m.size = size
	m.change = nil
	m.change = newCandChange()
	return nil
}

// LegacyCommit writes the change into base with legacy logic
func (m *CandidateCenter) LegacyCommit() error {
	size, err := m.base.commit(m.change, true)
	if err != nil {
		return err
	}
	m.size = size
	m.change = nil
	m.change = newCandChange()
	return nil
}

// IsDirty returns true if the candidate center is dirty
func (m *CandidateCenter) IsDirty() bool {
	return m.change.size() > 0
}

// ContainsName returns true if the map contains the candidate by name
func (m *CandidateCenter) ContainsName(name string) bool {
	if hit := m.change.containsName(name); hit {
		return true
	}

	if d, hit := m.base.getByName(name); hit {
		return !m.change.containsIdentifier(d.GetIdentifier())
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

	if d, hit := m.base.getByOwner(owner.String()); hit {
		return !m.change.containsIdentifier(d.GetIdentifier())
	}
	return false
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
		return !m.change.containsIdentifier(d.GetIdentifier())
	}
	return false
}

// ContainsSelfStakingBucket returns true if the map contains the self staking bucket index
func (m *CandidateCenter) ContainsSelfStakingBucket(index uint64) bool {
	if hit := m.change.containsSelfStakingBucket(index); hit {
		return true
	}

	if d, hit := m.base.getBySelfStakingIndex(index); hit {
		return !m.change.containsIdentifier(d.GetIdentifier())
	}
	return false
}

// GetByName returns the candidate by name
func (m *CandidateCenter) GetByName(name string) *Candidate {
	if d := m.change.getByName(name); d != nil {
		return d
	}

	if d, hit := m.base.getByName(name); hit && !m.change.containsIdentifier(d.GetIdentifier()) {
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

// GetByIdentifier returns the candidate by identifier
func (m *CandidateCenter) GetByIdentifier(identifier address.Address) *Candidate {
	if identifier == nil {
		return nil
	}

	if d := m.change.getByIdentifier(identifier); d != nil {
		return d
	}

	if d, hit := m.base.getByIdentifier(identifier.String()); hit {
		return d.Clone()
	}
	return nil
}

// GetBySelfStakingIndex returns the candidate by self-staking index
func (m *CandidateCenter) GetBySelfStakingIndex(index uint64) *Candidate {
	if d := m.change.getBySelfStakingIndex(index); d != nil {
		return d
	}

	if d, hit := m.base.getBySelfStakingIndex(index); hit && !m.change.containsIdentifier(d.GetIdentifier()) {
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

	if _, hit := m.base.getByIdentifier(d.GetIdentifier().String()); !hit {
		m.size++
	}
	// fmt.Printf("upsert done %+v\n", d)
	return nil
}

func (m *CandidateCenter) collision(d *Candidate) error {
	if err := m.change.collision(d); err != nil {
		return err
	}

	name, owner, oper, self := m.base.collision(d)
	if name != nil && !m.change.containsIdentifier(name) {
		return action.ErrInvalidCanName
	}
	if owner != nil && !m.change.containsIdentifier(owner) {
		return ErrInvalidOwner
	}

	if oper != nil && !m.change.containsIdentifier(oper) {
		return ErrInvalidOperator
	}

	if self != nil && !m.change.containsIdentifier(self) {
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

func (cc *candChange) clone() *candChange {
	clone := newCandChange()
	for _, d := range cc.candidates {
		clone.candidates = append(clone.candidates, d.Clone())
	}
	for k, v := range cc.dirty {
		clone.dirty[k] = v.Clone()
	}
	return clone
}

func (cc *candChange) size() int {
	return len(cc.dirty)
}

func (cc *candChange) view() CandidateList {
	if len(cc.dirty) == 0 {
		return nil
	}

	list := make(CandidateList, 0, len(cc.dirty))
	for _, d := range cc.dirty {
		list = append(list, d.Clone())
	}
	return list
}

func (cc *candChange) items() CandidateList {
	var retval CandidateList
	for _, c := range cc.candidates {
		retval = append(retval, c.Clone())
	}
	return retval
}

func (cc *candChange) containsName(name string) bool {
	for _, d := range cc.dirty {
		if name == d.Name {
			return true
		}
	}
	return false
}

func (cc *candChange) containsIdentifier(identifier address.Address) bool {
	if identifier == nil {
		return false
	}
	_, ok := cc.dirty[identifier.String()]
	return ok
}

func (cc *candChange) containsOwner(owner address.Address) bool {
	if owner == nil {
		return false
	}
	for _, d := range cc.dirty {
		if address.Equal(owner, d.Owner) {
			return true
		}
	}
	return false
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
		if d.isSelfStakeBucketSettled() && index == d.SelfStakeBucketIdx {
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

	for _, d := range cc.dirty {
		if address.Equal(owner, d.Owner) {
			return d.Clone()
		}
	}
	return nil
}

func (cc *candChange) getByIdentifier(identifier address.Address) *Candidate {
	if identifier == nil {
		return nil
	}
	if d, ok := cc.dirty[identifier.String()]; ok {
		return d.Clone()
	}
	return nil
}

func (cc *candChange) getBySelfStakingIndex(index uint64) *Candidate {
	for _, d := range cc.dirty {
		if d.isSelfStakeBucketSettled() && index == d.SelfStakeBucketIdx {
			return d.Clone()
		}
	}
	return nil
}

func (cc *candChange) upsert(d *Candidate) error {
	if err := d.Validate(); err != nil {
		return err
	}
	cc.candidates = append(cc.candidates, d)
	cc.dirty[d.GetIdentifier().String()] = d
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

//======================================
// candBase funcs
//======================================

func newCandBase() *candBase {
	return &candBase{
		nameMap:          make(map[string]*Candidate),
		ownerMap:         make(map[string]*Candidate),
		operatorMap:      make(map[string]*Candidate),
		identifierMap:    make(map[string]*Candidate),
		selfStkBucketMap: make(map[uint64]*Candidate),
	}
}

func (cb *candBase) size() int {
	cb.lock.RLock()
	defer cb.lock.RUnlock()
	return len(cb.identifierMap)
}

func (cb *candBase) all() CandidateList {
	cb.lock.RLock()
	defer cb.lock.RUnlock()
	if len(cb.identifierMap) == 0 {
		return nil
	}

	list := make(CandidateList, 0, len(cb.identifierMap))
	for _, d := range cb.identifierMap {
		list = append(list, d.Clone())
	}
	return list
}

func (cb *candBase) commit(change *candChange, keepAliasBug bool) (int, error) {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	if keepAliasBug {
		for _, v := range change.dirty {
			if err := v.Validate(); err != nil {
				return 0, err
			}
			d := v.Clone()
			cb.ownerMap[d.Owner.String()] = d
			cb.nameMap[d.Name] = d
			cb.operatorMap[d.Operator.String()] = d
			cb.identifierMap[d.GetIdentifier().String()] = d
			cb.selfStkBucketMap[d.SelfStakeBucketIdx] = d
		}
	} else {
		for _, v := range change.candidates {
			if err := v.Validate(); err != nil {
				return 0, err
			}
			d := v.Clone()
			if curr, ok := cb.identifierMap[d.GetIdentifier().String()]; ok {
				delete(cb.nameMap, curr.Name)
				delete(cb.operatorMap, curr.Operator.String())
				delete(cb.ownerMap, curr.Owner.String())
				delete(cb.selfStkBucketMap, curr.SelfStakeBucketIdx)
			}
			cb.identifierMap[d.GetIdentifier().String()] = d
			cb.ownerMap[d.Owner.String()] = d
			cb.nameMap[d.Name] = d
			cb.operatorMap[d.Operator.String()] = d
			if d.isSelfStakeBucketSettled() {
				cb.selfStkBucketMap[d.SelfStakeBucketIdx] = d
			}
		}
	}
	return len(cb.identifierMap), nil
}

func (cb *candBase) getByName(name string) (*Candidate, bool) {
	cb.lock.RLock()
	defer cb.lock.RUnlock()
	d, ok := cb.nameMap[name]
	return d, ok
}

func (cb *candBase) getByIdentifier(identifier string) (*Candidate, bool) {
	cb.lock.RLock()
	defer cb.lock.RUnlock()
	d, ok := cb.identifierMap[identifier]
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

func (cb *candBase) collision(d *Candidate) (address.Address, address.Address, address.Address, address.Address) {
	cb.lock.RLock()
	defer cb.lock.RUnlock()
	var name, owner, oper, self address.Address
	if c, hit := cb.nameMap[d.Name]; hit && !address.Equal(c.GetIdentifier(), d.GetIdentifier()) {
		name = c.GetIdentifier()
	}
	if c, hit := cb.ownerMap[d.Owner.String()]; hit && !address.Equal(c.GetIdentifier(), d.GetIdentifier()) {
		owner = c.GetIdentifier()
	}

	if c, hit := cb.operatorMap[d.Operator.String()]; hit && !address.Equal(c.GetIdentifier(), d.GetIdentifier()) {
		oper = c.GetIdentifier()
	}

	if c, hit := cb.selfStkBucketMap[d.SelfStakeBucketIdx]; d.isSelfStakeBucketSettled() && hit && !address.Equal(c.GetIdentifier(), d.GetIdentifier()) {
		self = c.GetIdentifier()
	}
	return name, owner, oper, self
}

func (cb *candBase) deleteByOwner(owner address.Address) {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	if d, hit := cb.ownerMap[owner.String()]; hit {
		delete(cb.nameMap, d.Name)
		delete(cb.ownerMap, d.Owner.String())
		delete(cb.operatorMap, d.Operator.String())
		delete(cb.selfStkBucketMap, d.SelfStakeBucketIdx)
		delete(cb.identifierMap, d.GetIdentifier().String())
	}
}
