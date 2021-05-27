// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package routine_test

import (
	"context"
	"testing"
	"time"

	"github.com/facebookgo/clock"
	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/pkg/routine"
)

func TestDelayTaskTimeout(t *testing.T) {
	c := make(chan bool)
	ctx := context.Background()
	ck := clock.NewMock()
	task := routine.NewDelayTask(func() { c <- true }, 100*time.Millisecond, routine.WithClock(ck))
	task.Start(ctx)
	defer func() {
		task.Stop(ctx)
	}()

	ck.Add(1 * time.Second)
	assert.True(t, <-c, "Do executed")
}

func TestDelayTaskStop(t *testing.T) {
	c := make(chan bool)
	ctx := context.Background()
	task := routine.NewDelayTask(func() { c <- true }, 100*time.Millisecond)
	task.Start(ctx)
	task.Stop(ctx)

	select {
	case <-c:
		t.Fail()
	case <-time.After(600 * time.Millisecond):
	}
}
