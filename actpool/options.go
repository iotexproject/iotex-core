// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"errors"
	"time"

	"github.com/facebookgo/clock"
)

// ActQueueOption is the option for actQueue.
type ActQueueOption interface {
	SetActQueueOption(*actQueue)
}

type clockOption struct{ c clock.Clock }

// WithClock returns an option to overwrite clock.
func WithClock(c clock.Clock) interface{ ActQueueOption } {
	return &clockOption{c}
}

func (o *clockOption) SetActQueueOption(aq *actQueue) { aq.clock = o.c }

type ttlOption struct{ ttl time.Duration }

// WithTimeOut returns an option to overwrite time out setting.
func WithTimeOut(ttl time.Duration) interface{ ActQueueOption } {
	return &ttlOption{ttl}
}

func (o *ttlOption) SetActQueueOption(aq *actQueue) { aq.ttl = o.ttl }

// WithStore is the option to set store encode and decode functions.
func WithStore(cfg StoreConfig, encode encodeAction, decode decodeAction) func(*actPool) error {
	return func(a *actPool) error {
		if encode == nil || decode == nil {
			return errors.New("encode and decode functions must be provided")
		}
		store, err := newActionStore(actionStoreConfig{
			Datadir: cfg.Datadir,
		}, encode, decode)
		if err != nil {
			return err
		}
		a.store = store
		return nil
	}
}
