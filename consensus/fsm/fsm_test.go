// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package fsm

import (
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

type RuleNoError struct{ NilTimeout }

func (r RuleNoError) Condition(ctx *Event) bool {
	return ctx.Err == nil
}

type RuleHasError struct{ NilTimeout }

func (r RuleHasError) Condition(ctx *Event) bool {
	return ctx.Err != nil
}

type EmptyHandler struct {
	NilTimeout
	hasError bool
	handled  bool
}

func (h *EmptyHandler) Handle(ctx *Event) {
	if h.hasError {
		ctx.Err = errors.New("error")
	}
	h.handled = true
}

// TestStateTransition123 tests the transition from 1 -> 2 -> 3 -> 1
//                                                       |
//                                                       +-err-> 1
func TestStateTransition123(t *testing.T) {
	sm := NewMachine("")
	handle2 := &EmptyHandler{}
	sm.AddTransition("1", "2", RuleNoError{})
	sm.AddTransition("2", "3", RuleNoError{})
	sm.AddTransition("2", "1", RuleHasError{})
	sm.AddTransition("3", "1", RuleNoError{})
	sm.AddState("1", &EmptyHandler{})
	sm.AddState("2", handle2)
	sm.AddState("3", &EmptyHandler{})

	err := sm.transitionTo(State("1")) // initial state
	assert.NoError(t, err)
	assert.Equal(t, "1", string(sm.CurrentState()))

	err = sm.HandleTransition(&Event{State: "1"})
	assert.NoError(t, err)
	assert.Equal(t, "2", string(sm.CurrentState()))

	err = sm.HandleTransition(&Event{State: "2"})
	assert.NoError(t, err)
	assert.Equal(t, "3", string(sm.CurrentState()))

	err = sm.HandleTransition(&Event{State: "3"})
	assert.NoError(t, err)
	assert.Equal(t, "1", string(sm.CurrentState()))

	err = sm.HandleTransition(&Event{State: "1"})
	assert.NoError(t, err)
	assert.Equal(t, "2", string(sm.CurrentState()))

	handle2.hasError = true
	err = sm.HandleTransition(&Event{State: "2"})
	assert.NoError(t, err)
	assert.Equal(t, "1", string(sm.CurrentState()))
}

// 1 -> 2 -> 3 so 1 cannot move to 3 directly
func TestCannotSkipTransition(t *testing.T) {
	sm := NewMachine("")
	err := sm.SetInitialState("1", &EmptyHandler{})
	assert.Nil(t, err)
	sm.AddState("2", &EmptyHandler{})
	sm.AddState("3", &EmptyHandler{})
	sm.AddTransition("1", "2", RuleNoError{})
	sm.AddTransition("2", "3", RuleNoError{})

	err = sm.HandleTransition(&Event{State: "3"})
	assert.Equal(t, ErrStateHandlerNotMatched, errors.Cause(err))
}

// initial -> 1 -> 3
//     |
//     +----> 2
// when in initial state, the state machine accepts 1 and 2 still.
func TestInitialStateAcceptsStatesForNeighbors(t *testing.T) {
	sm := NewMachine("")
	start := &EmptyHandler{}
	err := sm.SetInitialState("initial", start)
	assert.Nil(t, err)
	sm.AddState("1", &EmptyHandler{})
	sm.AddState("2", &EmptyHandler{})
	sm.AddState("3", &EmptyHandler{})
	sm.AddTransition("initial", "1", RuleNoError{})
	sm.AddTransition("initial", "2", RuleNoError{})
	sm.AddTransition("1", "3", RuleNoError{})

	// cannot handle 3 directly from initial
	assert.Equal(t, State("initial"), sm.CurrentState())
	err = sm.HandleTransition(&Event{State: "3"})
	assert.Error(t, err)

	// can handle 1 directly from initial
	assert.Equal(t, State("initial"), sm.CurrentState())
	err = sm.HandleTransition(&Event{State: "1"})
	assert.True(t, start.handled, "start handler called")
	assert.Equal(t, State("3"), sm.CurrentState())
	assert.NoError(t, err)
}

// initial -NoError-> 1(hasError) -NoError-> 2
// when in initial state, the state machine accepts 1. if 1 set error, stays in 1.
func TestInitialStateAcceptsNeighborAndStaysIfError(t *testing.T) {
	sm := NewMachine("")
	err := sm.SetInitialState("initial", &EmptyHandler{})
	assert.Nil(t, err)
	sm.AddState("1", &EmptyHandler{hasError: true})
	sm.AddState("2", &EmptyHandler{})
	sm.AddTransition("initial", "1", RuleNoError{})
	sm.AddTransition("1", "2", RuleNoError{})

	// can handle 1 directly from initial, but stays in 1 because there is error
	assert.Equal(t, State("initial"), sm.CurrentState())
	err = sm.HandleTransition(&Event{State: "1"})
	assert.Equal(t, State("1"), sm.CurrentState())
	assert.Equal(t, ErrNoTransitionApplied, errors.Cause(err))
}

//                    +-HasError-+
//                    |          |
//                   \|/         |
// initial -NoError-> 1(hasError) --NoError-> 2
// when in initial state, the state machine accepts 1. if 1 set error, stays in 1.
func TestInitialStateAcceptsNeighborCircle(t *testing.T) {
	sm := NewMachine("")
	err := sm.SetInitialState("initial", &EmptyHandler{})
	assert.Nil(t, err)
	sm.AddState("1", &EmptyHandler{hasError: true})
	sm.AddState("2", &EmptyHandler{})
	sm.AddTransition("initial", "1", RuleNoError{})
	sm.AddTransition("1", "2", RuleNoError{})
	sm.AddTransition("1", "1", RuleHasError{})

	// can handle 1 directly from initial, but stays in 1 because there is error
	assert.Equal(t, State("initial"), sm.CurrentState())
	err = sm.HandleTransition(&Event{State: "1"})
	assert.Equal(t, State("1"), sm.CurrentState())
	assert.NoError(t, err)
}

// TestStateTransitionToUnknownState
func TestStateTransitionToUnknownState(t *testing.T) {
	sm := NewMachine("")
	err := sm.transitionTo(State("blah"))
	assert.Equal(t, ErrStateUndefined, errors.Cause(err))
}

type TimeoutHandler struct{}

func (h *TimeoutHandler) TimeoutDuration() *time.Duration {
	d := time.Duration(100 * time.Millisecond)
	return &d
}

func (h *TimeoutHandler) Handle(ctx *Event) {
	ctx.Err = errors.New("error")
}

type RuleTimeout struct{}

func (r *RuleTimeout) Condition(ctx *Event) bool {
	return ctx.StateTimedOut
}

// initial -> 1 -> 2
// when state 1 timeout, the state will move to 2 automatically
func TestTimeoutTriggersTransition(t *testing.T) {
	sm := NewMachine("")
	err := sm.SetInitialState("initial", &EmptyHandler{})
	assert.Nil(t, err)
	sm.AddState("1", &TimeoutHandler{})
	sm.AddState("2", &EmptyHandler{hasError: true})
	sm.AddTransition("initial", "1", RuleNoError{})
	sm.AddTransition("1", "2", &RuleTimeout{})

	err = sm.HandleTransition(&Event{State: "1"})
	assert.Equal(t, ErrNoTransitionApplied, errors.Cause(err))
	assert.Equal(t, State("1"), sm.CurrentState())

	time.Sleep(400 * time.Millisecond)
	assert.Equal(t, State("2"), sm.CurrentState())
}

// initial -> 1 -> 2
// when state 1 -> 2 happens, the state 1's timeout is stopped.
func TestTransitionStopsTimeout(t *testing.T) {
	sm := NewMachine("")
	err := sm.SetInitialState("initial", &EmptyHandler{})
	assert.Nil(t, err)
	sm.AddState("1", &TimeoutHandler{})
	sm.AddState("2", &EmptyHandler{})
	sm.AddTransition("initial", "1", RuleNoError{})
	sm.AddTransition("1", "2", &RuleHasError{})

	err = sm.HandleTransition(&Event{State: "1"})
	assert.NoError(t, err)
	assert.Equal(t, State("2"), sm.CurrentState())
}

// initial -> 1 -> 2
// when state 1 timeout, the state will move to 2 automatically
func TestTimeoutStopsIfTransitionApplies(t *testing.T) {
	sm := NewMachine("")
	err := sm.SetInitialState("initial", &EmptyHandler{})
	assert.Nil(t, err)
	sm.AddState("1", &TimeoutHandler{})
	sm.AddState("2", &EmptyHandler{hasError: true})
	sm.AddTransition("initial", "1", RuleNoError{})
	sm.AddTransition("1", "2", &RuleTimeout{})

	err = sm.HandleTransition(&Event{State: "1"})
	assert.Equal(t, ErrNoTransitionApplied, errors.Cause(err))
	assert.Equal(t, State("1"), sm.CurrentState())

	time.Sleep(400 * time.Millisecond)
	assert.Equal(t, State("2"), sm.CurrentState())
}
