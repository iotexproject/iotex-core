// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

// stackView keeps its snapshots on a stack and hands a reverted snapshot's
// index out again, as the staking view does.
type stackView struct {
	value     int
	snapshots []int
}

func (v *stackView) Fork() View { return &stackView{value: v.value} }

func (v *stackView) Snapshot() int {
	v.snapshots = append(v.snapshots, v.value)
	return len(v.snapshots) - 1
}

func (v *stackView) Revert(i int) error {
	if i < 0 || i >= len(v.snapshots) {
		return errors.Errorf("invalid snapshot index %d", i)
	}
	v.value = v.snapshots[i]
	v.snapshots = v.snapshots[:i]
	return nil
}

func (v *stackView) Commit(context.Context, StateManager) error { return nil }

func TestViews_Revert(t *testing.T) {
	newViews := func() (*views, *stackView) {
		sv := &stackView{}
		vs := NewViews().(*views)
		vs.Write("stack", sv)
		return vs, sv
	}

	t.Run("inner then outer", func(t *testing.T) {
		r := require.New(t)
		vs, sv := newViews()
		outer := vs.Snapshot()
		sv.value = 1
		inner := vs.Snapshot()
		sv.value = 2
		r.NoError(vs.Revert(inner))
		r.Equal(1, sv.value)
		r.NoError(vs.Revert(outer))
		r.Equal(0, sv.value)
		r.Empty(vs.snapshots)
	})

	t.Run("reverted snapshot is consumed", func(t *testing.T) {
		r := require.New(t)
		vs, sv := newViews()
		id := vs.Snapshot()
		sv.value = 1
		r.NoError(vs.Revert(id))
		r.ErrorContains(vs.Revert(id), "already been reverted")
	})

	t.Run("reverted snapshot does not alias a later one", func(t *testing.T) {
		r := require.New(t)
		vs, sv := newViews()
		id := vs.Snapshot()
		sv.value = 1
		r.NoError(vs.Revert(id))
		// the view hands index 0 out again, for a state where value is 3
		sv.value = 3
		later := vs.Snapshot()
		r.NotEqual(id, later)
		sv.value = 4
		r.Error(vs.Revert(id))
		r.Equal(4, sv.value)
	})

	t.Run("snapshots taken after the reverted one are released", func(t *testing.T) {
		r := require.New(t)
		vs, _ := newViews()
		first := vs.Snapshot()
		second := vs.Snapshot()
		third := vs.Snapshot()
		r.Len(vs.snapshots, 3)
		r.NoError(vs.Revert(first))
		r.Empty(vs.snapshots)
		r.ErrorContains(vs.Revert(second), "invalid snapshot id")
		r.ErrorContains(vs.Revert(third), "invalid snapshot id")
	})
}
