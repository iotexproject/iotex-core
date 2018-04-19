// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package routine_test

import (
	"testing"
	"time"

	"github.com/iotexproject/iotex-core/common/routine"
	"github.com/stretchr/testify/assert"
)

type MockHandler struct {
	Done bool
}

func (h *MockHandler) Do() {
	h.Done = true
}

func TestTimeoutTaskTimeout(t *testing.T) {
	h := &MockHandler{}
	task := routine.NewTimeoutTask(h, 100*time.Millisecond)
	task.Init()
	task.Start()
	defer func() {
		task.Stop()
	}()

	time.Sleep(600 * time.Millisecond)
	assert.True(t, h.Done, "Do executed")
}

func TestTimeoutTaskStop(t *testing.T) {
	h := &MockHandler{}
	task := routine.NewTimeoutTask(h, 100*time.Millisecond)
	task.Init()
	task.Start()
	task.Stop()

	time.Sleep(600 * time.Millisecond)
	assert.False(t, h.Done, "Do not executed because stopped")
}
