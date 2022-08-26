// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package consensus

import (
	"time"

	fsm "github.com/iotexproject/go-fsm"
)

// Event defines the event used in the fsm
type Event struct {
	fsm.Event
	height       uint64
	round        uint32
	eventType    fsm.EventType
	creationTime time.Time
	data         interface{}
}

// NewEvent creates a new consensus event
func NewEvent(
	eventType fsm.EventType,
	data interface{},
	height uint64,
	round uint32,
	creationTime time.Time,
) *Event {
	return &Event{
		data:         data,
		height:       height,
		round:        round,
		eventType:    eventType,
		creationTime: creationTime,
	}
}

// Height is the height of the event
func (e *Event) Height() uint64 {
	return e.height
}

// Round is the round of the event
func (e *Event) Round() uint32 {
	return e.round
}

// Type returns the event type
func (e *Event) Type() fsm.EventType {
	return e.eventType
}

// Timestamp is the creation time of the event
func (e *Event) Timestamp() time.Time {
	return e.creationTime
}

// Data returns the data of the event
func (e *Event) Data() interface{} {
	return e.data
}
