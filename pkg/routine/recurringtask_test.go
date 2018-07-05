// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package routine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type MockHandler struct {
	Count uint
}

func (h *MockHandler) Do() {
	h.Count++
}

func TestRecurringTask(t *testing.T) {
	h := &MockHandler{Count: 0}
	ctx := context.Background()
	task := NewRecurringTask(h, 100*time.Millisecond)
	task.Start(ctx)
	defer func() {
		task.Stop(ctx)
	}()

	time.Sleep(600 * time.Millisecond)
	assert.True(t, h.Count >= 5)
}
