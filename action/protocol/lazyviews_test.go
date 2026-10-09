// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type testCounterView struct {
	value     int
	snapshots []int
}

func (v *testCounterView) Fork() View {
	return &testCounterView{value: v.value}
}

func (v *testCounterView) Snapshot() int {
	v.snapshots = append(v.snapshots, v.value)
	return len(v.snapshots) - 1
}

func (v *testCounterView) Revert(id int) error {
	v.value = v.snapshots[id]
	v.snapshots = v.snapshots[:id]
	return nil
}

func (v *testCounterView) Commit(context.Context, StateManager) error { return nil }

func TestLazyViewsRevertToSnapshotBeforeLoad(t *testing.T) {
	r := require.New(t)
	loaded := 0
	lv := NewLazyViews(func() Views {
		loaded++
		vs := NewViews()
		vs.Write("counter", &testCounterView{})
		return vs
	})

	// snapshot taken before the views are loaded
	sid := lv.Snapshot()
	v, err := lv.Read("counter")
	r.NoError(err)
	r.Equal(1, loaded)
	v.(*testCounterView).value = 10

	r.NoError(lv.Revert(sid))
	v, err = lv.Read("counter")
	r.NoError(err)
	r.Equal(0, v.(*testCounterView).value)

	// snapshot taken after the views are loaded
	v.(*testCounterView).value = 5
	sid = lv.Snapshot()
	v.(*testCounterView).value = 7
	r.NoError(lv.Revert(sid))
	v, err = lv.Read("counter")
	r.NoError(err)
	r.Equal(5, v.(*testCounterView).value)
}
