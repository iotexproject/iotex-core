// Copyright (c) 2021 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package lifecycle

import (
	"sync/atomic"

	"github.com/pkg/errors"
)

const (
	_notReady = 0
	_ready    = 1
)

// vars
var (
	ErrWrongState = errors.New("service is in wrong state")
)

// Readiness is a thread-safe struct to indicate a service's status
type Readiness struct {
	ready int32
}

// TurnOn sets the service to ready (can accept service request)
func (r *Readiness) TurnOn() error {
	if atomic.CompareAndSwapInt32(&r.ready, _notReady, _ready) {
		return nil
	}
	return ErrWrongState
}

// TurnOff sets the service to not ready (initial state)
func (r *Readiness) TurnOff() error {
	if atomic.CompareAndSwapInt32(&r.ready, _ready, _notReady) {
		return nil
	}
	return ErrWrongState
}

// IsReady returns whether the service is ready (can accept service request)
func (r *Readiness) IsReady() bool {
	return atomic.LoadInt32(&r.ready) == _ready
}
