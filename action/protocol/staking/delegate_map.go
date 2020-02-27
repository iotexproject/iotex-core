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
	// DelegateMap is a map of delegates using owner's 12-byte name as key
	DelegateMap map[CandName]*Delegate
)

// Contains returns true if the map contains the delegate by name
func (m DelegateMap) Contains(name CandName) bool {
	_, ok := m[name]
	return ok
}

// Get returns the delegate by name
func (m DelegateMap) Get(name CandName) *Delegate {
	return m[name]
}

// Put writes the delegate into map, will overwrite if name already exists
func (m DelegateMap) Put(d *Delegate) error {
	if err := d.Validate(); err != nil {
		return err
	}

	m[d.CanName] = d
	return nil
}

// Delete deletes the delegate by name
func (m DelegateMap) Delete(name CandName) {
	delete(m, name)
}

// Serialize serializes a DelegateMap to bytes
func (m DelegateMap) Serialize() ([]byte, error) {
	l := make(DelegateList, 0, len(m))
	for _, v := range m {
		l = append(l, v)
	}
	sort.Sort(l)
	return proto.Marshal(l.toProto())
}

// Deserialize deserializes bytes to DelegateMap
func (m DelegateMap) Deserialize(buf []byte) error {
	pb := &stakingpb.Delegates{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return errors.Wrap(err, "failed to unmarshal delegate list")
	}

	for k := range m {
		delete(m, k)
	}

	for _, v := range pb.Delegates {
		m[ToCandName(v.CanName)] = fromProto(v)
	}
	return nil
}
