// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package routine_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/common/routine"
)

func TestDelayTaskTimeout(t *testing.T) {
	c := make(chan bool)
	task := routine.NewDelayTask(func() { c <- true }, 100*time.Millisecond)
	task.Init()
	task.Start()
	defer func() {
		task.Stop()
	}()

	time.Sleep(600 * time.Millisecond)
	assert.True(t, <-c, "Do executed")
}

func TestDelayTaskStop(t *testing.T) {
	c := make(chan bool)
	task := routine.NewDelayTask(func() { c <- true }, 100*time.Millisecond)
	task.Init()
	task.Start()
	task.Stop()

	select {
	case <-c:
		t.Fail()
	case <-time.After(600 * time.Millisecond):
	}
}
