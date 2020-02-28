// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"sort"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol/staking/stakingpb"
)

type (
	// CandidateMap is a map of candidates using owner's 12-byte name as key
	CandidateMap map[string]*Candidate
)

// Contains returns true if the map contains the candidate by name
func (m CandidateMap) Contains(name string) bool {
	_, ok := m[name]
	return ok
}

// Get returns the candidate by name
func (m CandidateMap) Get(name string) *Candidate {
	return m[name]
}

// Put writes the candidate into map, will overwrite if name already exists
func (m CandidateMap) Put(d *Candidate) error {
	m[d.Name] = d
	return nil
}

// Delete deletes the candidate by name
func (m CandidateMap) Delete(name string) {
	delete(m, name)
}

// Serialize serializes a CandidateMap to bytes
func (m CandidateMap) Serialize() ([]byte, error) {
	l := make(CandidateList, 0, len(m))
	for _, v := range m {
		l = append(l, v)
	}
	sort.Sort(l)
	lpb, err := l.toProto()
	if err != nil {
		return nil, err
	}
	return proto.Marshal(lpb)
}

// Deserialize deserializes bytes to CandidateMap
func (m CandidateMap) Deserialize(buf []byte) error {
	pb := &stakingpb.Candidates{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return errors.Wrap(err, "failed to unmarshal candidate list")
	}

	for k := range m {
		delete(m, k)
	}

	for _, v := range pb.Candidates {
		c := &Candidate{}
		if err := c.fromProto(v); err != nil {
			return err
		}
		m[v.Name] = c
	}
	return nil
}
