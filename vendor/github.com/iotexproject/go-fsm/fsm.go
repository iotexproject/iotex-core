// Copyright 2018 Zhijie Shen
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fsm

import (
	"sync"

	"github.com/pkg/errors"
)

var (
	// ErrBuild represents the error of building an FSM.
	ErrBuild = errors.New("error when building an FSM")
	// ErrTransitionNotFound indicates that there doesn't exist a transition defined on the source state and the event.
	ErrTransitionNotFound = errors.New("unable to find a valid transition")
	// ErrInvalidTransition indicates that the actual destination state after transition is not defined.
	ErrInvalidTransition = errors.New("invalid transition")
)

// State represents the state and it is a string.
type State string

// EventType represents a event type and it is a string.
type EventType string

// Event is the interface of the events that could be handled by an FSM
type Event interface {
	// Type returns the event type
	Type() EventType
}

// Transition is the interface of the transition that would happen on a certain state when receiving a certain event.
// The transition returns a destination state where an FSM should transit into or an error.
type Transition func(Event) (State, error)

// FSM is the interface of an FSM (finite state machine). It allows to define the transition logic and destinations
// after the transition, and intake the event to trigger the transition. The event handling is synchronized, so that it
// guarantee always processing one event at any time. An FSM must have exactly one initial state.
type FSM interface {
	// CurrentState returns the current state.
	CurrentState() State
	// Handle handles an event and return error if there is any.
	Handle(Event) error
}

// Builder is an FSM builder to help construct an FSM.
type Builder struct {
	states map[State]bool
	td     map[State]map[EventType]transAndDsts
}

// NewBuilder creates an FSM builder instance with empty setup.
func NewBuilder() *Builder {
	return &Builder{
		states: make(map[State]bool),
		td:     make(map[State]map[EventType]transAndDsts),
	}
}

// AddInitialState adds an initial state
func (b *Builder) AddInitialState(s State) *Builder {
	if _, ok := b.states[s]; !ok {
		b.states[s] = true
	}
	return b
}

// AddStates adds the non-initial state(s)
func (b *Builder) AddStates(states ...State) *Builder {
	for _, s := range states {
		if _, ok := b.states[s]; !ok {
			b.states[s] = false
		}
	}
	return b
}

// AddTransition adds a transition setup, including the source state, the event to trigger the transition, the
// transition callback, and the legal destination transitions.
func (b *Builder) AddTransition(src State, et EventType, trans Transition, dsts []State) *Builder {
	tdPerState, ok := b.td[src]
	if !ok {
		tdPerState = make(map[EventType]transAndDsts)
		b.td[src] = tdPerState
	}
	if _, ok := tdPerState[et]; !ok {
		dstsCpy := make([]State, len(dsts))
		copy(dstsCpy, dsts)
		tdPerState[et] = transAndDsts{
			trans: trans,
			dsts:  dstsCpy,
		}
	}
	return b
}

// Build builds an FSM instance based on the configured states and transition setup.
func (b *Builder) Build() (FSM, error) {
	m := &fsm{
		td: make(map[State]map[EventType]transAndDsts),
	}
	initStateFound := false
	for s, init := range b.states {
		m.td[s] = nil
		if !init {
			continue
		}
		if initStateFound {
			return nil, errors.Wrap(ErrBuild, "more than one initial state defined")
		}
		m.state = s
		initStateFound = true
	}
	if !initStateFound {
		return nil, errors.Wrap(ErrBuild, "no initial state defined")
	}
	for s, tdPerState := range b.td {
		if _, ok := m.td[s]; !ok {
			return nil, errors.Wrapf(ErrBuild, "transition from a undefined state %s", s)
		}
		for _, td := range tdPerState {
			for _, dst := range td.dsts {
				if _, ok := m.td[dst]; !ok {
					return nil, errors.Wrapf(ErrBuild, "transition to a undefined state %s", dst)
				}
			}
		}
		m.td[s] = tdPerState
	}
	return m, nil
}

// fsm implements FSM interface
type fsm struct {
	mutex sync.RWMutex
	state State
	td    map[State]map[EventType]transAndDsts
}

func (m *fsm) CurrentState() State {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.state
}

func (m *fsm) Handle(e Event) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	tdPerState, ok := m.td[m.state]
	if !ok {
		return errors.Wrapf(
			ErrTransitionNotFound,
			"transition and destinations are found for state %s",
			m.state,
		)
	}
	td, ok := tdPerState[e.Type()]
	if !ok {
		return errors.Wrapf(
			ErrTransitionNotFound,
			"transition and destinations are found for state %s, event %s",
			m.state,
			e.Type(),
		)
	}
	dst, err := td.trans(e)
	if err != nil {
		return err
	}
	for _, d := range td.dsts {
		if dst == d {
			m.state = dst
			return nil
		}
	}
	return errors.Wrapf(ErrInvalidTransition, "undefined transition from state %s to state %s", m.state, dst)
}

type transAndDsts struct {
	trans Transition
	dsts  []State
}
