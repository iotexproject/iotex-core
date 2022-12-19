// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package routine_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/facebookgo/clock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/pkg/routine"
)

type MockHandler struct {
	Count uint
	mu    sync.RWMutex
}

func (h *MockHandler) Do() {
	h.mu.Lock()
	h.Count++
	h.mu.Unlock()
}

func TestRecurringTask(t *testing.T) {
	require := require.New(t)
	h := &MockHandler{Count: 0}
	ctx := context.Background()
	ck := clock.NewMock()
	task := routine.NewRecurringTask(h.Do, 100*time.Millisecond, routine.WithClock(ck))
	require.Error(task.Stop(ctx))
	task.Start(ctx)
	ck.Add(600 * time.Millisecond)
	task.Stop(ctx)
	h.mu.RLock()
	require.True(h.Count >= 5)
	h.mu.RUnlock()
}
