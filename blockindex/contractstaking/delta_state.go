// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import "github.com/pkg/errors"

const (
	// deltaState constants
	// deltaStateUnchanged is the zero-value of the type deltaState
	deltaStateUnchanged deltaState = iota
	deltaStateAdded
	deltaStateRemoved
	deltaStateModified
)

type deltaState int

var (
	deltaStateTransferMap = map[deltaState]map[deltaAction]deltaState{
		deltaStateUnchanged: {
			deltaActionAdd:    deltaStateAdded,
			deltaActionRemove: deltaStateRemoved,
			deltaActionModify: deltaStateModified,
		},
		deltaStateAdded: {
			deltaActionModify: deltaStateAdded,
			deltaActionRemove: deltaStateUnchanged,
		},
		deltaStateModified: {
			deltaActionModify: deltaStateModified,
			deltaActionRemove: deltaStateRemoved,
		},
	}
)

func (s deltaState) Transfer(act deltaAction) (deltaState, error) {
	if _, ok := deltaStateTransferMap[s]; !ok {
		return s, errors.Errorf("invalid delta state %d", s)
	}
	if _, ok := deltaStateTransferMap[s][act]; !ok {
		return s, errors.Errorf("invalid delta action %d on state %d", act, s)
	}
	return deltaStateTransferMap[s][act], nil
}
