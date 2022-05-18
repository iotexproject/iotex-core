// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package consensusfsm

import (
	"time"

	fsm "github.com/iotexproject/go-fsm"
)

// ConsensusEvent defines the event used in the fsm
type ConsensusEvent struct {
	fsm.Event
	height       uint64
	round        uint32
	eventType    fsm.EventType
	creationTime time.Time
	data         interface{}
}

// NewConsensusEvent creates a new consensus event
func NewConsensusEvent(
	eventType fsm.EventType,
	data interface{},
	height uint64,
	round uint32,
	creationTime time.Time,
) *ConsensusEvent {
	return &ConsensusEvent{
		data:         data,
		height:       height,
		round:        round,
		eventType:    eventType,
		creationTime: creationTime,
	}
}

// Height is the height of the event
func (e *ConsensusEvent) Height() uint64 {
	return e.height
}

// Round is the round of the event
func (e *ConsensusEvent) Round() uint32 {
	return e.round
}

// Type returns the event type
func (e *ConsensusEvent) Type() fsm.EventType {
	return e.eventType
}

// Timestamp is the creation time of the event
func (e *ConsensusEvent) Timestamp() time.Time {
	return e.creationTime
}

// Data returns the data of the event
func (e *ConsensusEvent) Data() interface{} {
	return e.data
}
