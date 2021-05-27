// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package routine

import (
	"github.com/facebookgo/clock"
)

type clockOption struct{ c clock.Clock }

// WithClock set a clock to a task.
func WithClock(c clock.Clock) interface {
	RecurringTaskOption
	DelayTaskOption
} {
	return &clockOption{c}
}

func (o *clockOption) SetRecurringTaskOption(r *RecurringTask) {
	r.clock = o.c
}

func (o *clockOption) SetDelayTaskOption(d *DelayTask) {
	d.clock = o.c
}
