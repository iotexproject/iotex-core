// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actpool

import (
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
