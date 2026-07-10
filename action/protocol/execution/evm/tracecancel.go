// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"context"
	"sync"
)

type traceCancellerCtxKey struct{}

// TraceCanceller collects the cancel functions of EVMs created while serving a
// traced simulation (debug_trace*), so that a trace-timeout watchdog can abort
// opcode execution. Stopping the tracer alone only stops result collection —
// the EVM would keep executing until gas exhaustion; upstream geth pairs
// tracer.Stop with EVM.Cancel for the same reason.
//
// It is safe for concurrent use: the watchdog goroutine calls Cancel while the
// execution goroutine registers EVMs. Once Cancel has fired, any EVM
// registered afterwards (e.g. later transactions of a block trace) is
// cancelled immediately at registration.
type TraceCanceller struct {
	mu      sync.Mutex
	fired   bool
	cancels []func()
}

// NewTraceCanceller creates an empty TraceCanceller.
func NewTraceCanceller() *TraceCanceller {
	return &TraceCanceller{}
}

// register adds an EVM cancel function; if Cancel has already fired, the
// function is invoked immediately.
func (tc *TraceCanceller) register(cancel func()) {
	tc.mu.Lock()
	fired := tc.fired
	if !fired {
		tc.cancels = append(tc.cancels, cancel)
	}
	tc.mu.Unlock()
	if fired {
		cancel()
	}
}

// Cancel aborts every EVM registered so far and marks the canceller fired so
// that later registrations abort immediately. Idempotent.
func (tc *TraceCanceller) Cancel() {
	tc.mu.Lock()
	tc.fired = true
	cancels := tc.cancels
	tc.cancels = nil
	tc.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

// WithTraceCanceller attaches a TraceCanceller to the context; executeInEVM
// registers every EVM it creates with it. Only trace/simulation entry points
// should set this — consensus block processing must never carry one.
func WithTraceCanceller(ctx context.Context, tc *TraceCanceller) context.Context {
	return context.WithValue(ctx, traceCancellerCtxKey{}, tc)
}

// GetTraceCanceller returns the TraceCanceller attached to the context, or nil.
func GetTraceCanceller(ctx context.Context) *TraceCanceller {
	tc, _ := ctx.Value(traceCancellerCtxKey{}).(*TraceCanceller)
	return tc
}
