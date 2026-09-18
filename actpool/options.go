// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"errors"
	"time"

	"github.com/facebookgo/clock"

	"github.com/iotexproject/iotex-core/v2/action"
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

// WithBundlePool is the option to set bundle pool.
func WithBundlePool(bp *BundlePool) func(*actPool) error {
	return func(ap *actPool) error {
		if bp == nil {
			return errors.New("bundle pool cannot be nil")
		}
		ap.bundlePool = bp
		return nil
	}
}

// WithPrivateValidator adds a validator that runs on the Add path only.
//
// Private validators are deliberately excluded from actPool.Validate, which
// block validation reaches via the bundle pool. Anything whose rejection would
// change a block's validity -- rather than merely keeping an action out of this
// node's pool -- belongs here rather than in AddActionEnvelopeValidators.
func WithPrivateValidator(v action.SealedEnvelopeValidator) func(*actPool) error {
	return func(ap *actPool) error {
		if v == nil {
			return errors.New("private validator cannot be nil")
		}
		ap.privateValidators = append(ap.privateValidators, v)
		return nil
	}
}

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
