// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"sort"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol/staking/stakingpb"
)

type (
	// CandidateCenter is a struct to manage the candidates
	CandidateCenter struct {
		nameMap     map[string]*Candidate
		ownerMap    map[address.Address]*Candidate
		operatorMap map[address.Address]*Candidate
	}
)

// NewCandidateCenter creates an instance of CandidateCenter
func NewCandidateCenter() *CandidateCenter {
	return &CandidateCenter{
		nameMap:     make(map[string]*Candidate),
		ownerMap:    make(map[address.Address]*Candidate),
		operatorMap: make(map[address.Address]*Candidate),
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
	_, ok := m.ownerMap[owner]
	return ok
}

// ContainsOperator returns true if the map contains the candidate by operator
func (m CandidateCenter) ContainsOperator(operator address.Address) bool {
	_, ok := m.operatorMap[operator]
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
	if d, ok := m.ownerMap[owner]; ok {
		return d.Clone()
	}
	return nil
}

// Put writes the candidate into map
func (m CandidateCenter) Put(d *Candidate) error {
	m.nameMap[d.Name] = d
	m.ownerMap[d.Owner] = d
	m.operatorMap[d.Operator] = d
	return nil
}

// Delete deletes the candidate by name
func (m CandidateCenter) Delete(owner address.Address) {
	d, ok := m.ownerMap[owner]
	if !ok {
		return
	}

	delete(m.nameMap, d.Name)
	delete(m.ownerMap, d.Owner)
	delete(m.operatorMap, d.Operator)
}

// Serialize serializes a CandidateCenter to bytes
func (m CandidateCenter) Serialize() ([]byte, error) {
	l := make(CandidateList, 0, len(m.nameMap))
	for _, v := range m.nameMap {
		l = append(l, v)
	}
	sort.Sort(l)
	lpb, err := l.toProto()
	if err != nil {
		return nil, err
	}
	return proto.Marshal(lpb)
}

// Deserialize deserializes bytes to CandidateCenter
func (m CandidateCenter) Deserialize(buf []byte) error {
	pb := &stakingpb.Candidates{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return errors.Wrap(err, "failed to unmarshal candidate list")
	}

	m.nameMap = nil
	m.ownerMap = nil
	m.operatorMap = nil
	m.nameMap = make(map[string]*Candidate)
	m.ownerMap = make(map[address.Address]*Candidate)
	m.operatorMap = make(map[address.Address]*Candidate)

	for _, v := range pb.Candidates {
		c := &Candidate{}
		if err := c.fromProto(v); err != nil {
			return err
		}
		m.nameMap[c.Name] = c
		m.ownerMap[c.Owner] = c
		m.operatorMap[c.Operator] = c
	}
	return nil
}
