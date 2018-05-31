// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package fsm

import (
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/common/routine"
	"github.com/iotexproject/iotex-core/logger"
)

var (
	// ErrMachineNotInitialized is the error thatthe  state machine is not initialized.
	ErrMachineNotInitialized = errors.New("state machine is not initialized")
	// ErrTransitionNotPermitted is the error that the state transition is not permitted.
	ErrTransitionNotPermitted = errors.New("state transition is not permitted")
	// ErrStateUndefined is the error that the state is undefined
	ErrStateUndefined = errors.New("state is undefined")
	// ErrStateHandlerNotMatched is the error that the current event is not matched to state handler
	ErrStateHandlerNotMatched = errors.New("event is not matched to state handler")
	// ErrNoTransitionApplied is the error that no state transition is applied
	ErrNoTransitionApplied = errors.New("no state transition is applied")
)

// State is the machine state name.
type State string

// TransitionRuleMap is a set of map of transition destination states to rules.
type TransitionRuleMap map[State]Rule

// Copy clones the TransitionRuleMap.
func (trs TransitionRuleMap) Copy() TransitionRuleMap {
	clone := make(TransitionRuleMap)

	for rule, value := range trs {
		clone[rule] = value
	}

	return clone
}

// Event is holding request event info across the handler and the rule.
type Event struct {
	Err           error
	State         State
	StateTimedOut bool
	Block         *blockchain.Block
	BlockHash     *common.Hash32B
	SenderAddr    net.Addr
	ExpireAt      *time.Time
	SeenState     State
}

// Rule condition is evaluated when state handler is called.
type Rule interface {
	Condition(event *Event) bool
}

// Handler handles events for the state
// The difference between state handler and rule is that
//   1. rule dose not handle state event, it's just a transition from one state to another.
//   2. rule must succeed. even it has side-effects, no matter they fail or not, the transition is deterministic to
//      the destination state.
type Handler interface {
	Handle(event *Event)

	// TimeoutDuration returns the timeout of the state handler.
	// If it returns a duration, after the duration and the state is still not
	// If it returns nil, then no timeout task is set.
	TimeoutDuration() *time.Duration
}

// NilTimeout is the base handler struct that has no timeout
type NilTimeout struct{}

// TimeoutDuration returns the duration for timeout
func (h *NilTimeout) TimeoutDuration() *time.Duration {
	return nil
}

// Machine is the state machine.
type Machine struct {
	name         string
	state        State
	initialState State
	mu           sync.RWMutex
	toTask       *routine.TimeoutTask

	transitions map[State]TransitionRuleMap
	handlers    map[State]Handler
}

// NewMachine returns a state machine instance
func NewMachine(name string) Machine {
	sm := Machine{name: name}

	sm.handlers = make(map[State]Handler)
	sm.transitions = make(map[State]TransitionRuleMap)

	return sm
}

// SetInitialState sets the initial state which not only handles request for itself
// but also accepts state types for all direct neighbor states
func (m *Machine) SetInitialState(state State, handler Handler) error {
	m.AddState(state, handler)
	err := m.transitionAndSetupTimeout(state, &Event{State: "EMPTY"})
	if err != nil {
		logger.Error().Msg("failed to SetInitialState: cannot transit to initial state")
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.initialState = state
	return nil
}

// CurrentState returns the machine's current state. It returns "" when not initialized.
func (m *Machine) CurrentState() State {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.state
}

// transitionRules returns the allowed states to transit to and corresponding rules.
func (m *Machine) transitionRules(state State) (TransitionRuleMap, error) {
	if m.transitions == nil {
		return nil, errors.Wrap(ErrMachineNotInitialized, "the machine has not been fully initialized")
	}

	if _, ok := m.transitions[state]; !ok {
		return nil, errors.Wrapf(ErrStateUndefined, "state %s has not been registered", state)
	}

	return m.transitions[state].Copy(), nil
}

// AddTransition is a function for adding a valid state transition to the machine.
func (m *Machine) AddTransition(src State, dest State, rule Rule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.transitions == nil {
		m.transitions = make(map[State]TransitionRuleMap)
	}

	if m.transitions[src] == nil {
		m.transitions[src] = make(TransitionRuleMap)
	}
	m.transitions[src][dest] = rule

	if m.transitions[dest] == nil {
		m.transitions[dest] = make(TransitionRuleMap)
	}

	return nil
}

// AddState add the state and the handler.
func (m *Machine) AddState(state State, handler Handler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.handlers[state] = handler
	if m.transitions == nil {
		m.transitions = make(map[State]TransitionRuleMap)
	}
	if _, ok := m.transitions[state]; !ok {
		m.transitions[state] = make(TransitionRuleMap)
	}
}

// returns whether the event state matches the current state, or, if at starting state, event matches neighboring state
func (m *Machine) isExpectedState(event *Event) bool {
	if event.State == m.state {
		return true
	}

	return m.tryMoveToStartingState(event)
}

// returns true if the fsm is in the initial state and event is meant for a neighboring state; if so, transitions
func (m *Machine) tryMoveToStartingState(event *Event) bool {
	if m.state == m.initialState {
		trm, err := m.transitionRules(m.initialState)
		if err != nil {
			return false
		}
		for dest, rule := range trm {
			if dest != event.State {
				continue
			}

			// now the event is targeting a starting state.
			if !rule.Condition(event) {
				return false
			}

			err := m.transitionAndSetupTimeout(dest, event)
			if err != nil {
				return false
			}
			h := m.handlers[m.initialState]
			h.Handle(event)
			return true
		}
	}
	return false
}

// automatically transitions given an event; assumes event state is correct
func (m *Machine) autoTransition(ctx *Event) error {
	trm, err := m.transitionRules(ctx.State)
	if err != nil {
		return err
	}

	// try each outgoing rule
	for dest, rule := range trm {
		matched := rule.Condition(ctx)
		if matched {
			err = m.transitionAndSetupTimeout(dest, ctx)
			return err
		}
	}

	return errors.Wrap(ErrNoTransitionApplied, "cannot make auto transition")
}

// HandleTransition handles the event and transitions to the next state if it can
func (m *Machine) HandleTransition(ctx *Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// not current state or (current state is initial but incoming state is not for neighbors)
	if !m.isExpectedState(ctx) {
		return errors.Wrap(ErrStateHandlerNotMatched, "the event does not match the current state")
	}

	handler, ok := m.handlers[ctx.State]
	if !ok {
		return errors.Wrapf(ErrStateUndefined, "the machine has no handler for state '%+v'", ctx.State)
	}

	// state handler handles the input ctx
	handler.Handle(ctx)

	// auto transition to next state
	err := m.autoTransition(ctx)
	// auto transition error
	if err != nil {
		return err
	}

	return nil
}

// transitionTo makes a transition to the destination state if it exists
func (m *Machine) transitionTo(dest State) error {
	if m.transitions == nil {
		return errors.Wrap(ErrMachineNotInitialized, "the machine has no states added")
	}

	if m.state == "" {
		if _, ok := m.transitions[dest]; !ok {
			return errors.Wrap(ErrStateUndefined, "the initial state has not been defined within the machine")
		}

		m.state = dest
		return nil
	}

	if _, ok := m.transitions[m.state][dest]; !ok {
		return errors.Wrapf(ErrTransitionNotPermitted, "transition from state %s dest %s is not permitted", m.state, dest)
	}

	if _, ok := m.transitions[dest]; !ok {
		return errors.Wrapf(ErrStateUndefined, "state %s has not been registered", dest)
	}

	logger.Debug().
		Str("name", m.name).
		Str("src", string(m.state)).
		Str("dst", string(dest)).
		Msg("state transition")
	m.state = dest

	return nil
}

func (m *Machine) transitionAndSetupTimeout(dest State, ctx *Event) error {
	curState := m.state
	err := m.transitionTo(dest)
	if err != nil {
		return err
	}

	if dest != curState {
		m.tryStopTimeout()
		m.tryStartTimeout(dest)
	}
	return nil
}

type task struct {
	do func()
}

func (t *task) Do() {
	t.do()
}

func (m *Machine) tryStartTimeout(state State) {
	handler := m.handlers[state]
	if handler.TimeoutDuration() == nil {
		return
	}
	m.toTask = routine.NewTimeoutTask(
		&task{
			do: func() {
				ctx := &Event{
					StateTimedOut: true,
					State:         state,
				}
				m.autoTransition(ctx)
			},
		},
		*handler.TimeoutDuration(),
	)
	m.toTask.Start()
}

func (m *Machine) tryStopTimeout() {
	if m.toTask != nil {
		m.toTask.Stop()
		m.toTask = nil
	}
}
