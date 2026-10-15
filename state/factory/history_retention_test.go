// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsHistoryPruned(t *testing.T) {
	r := require.New(t)
	for _, c := range []struct {
		height, tip, retention uint64
		pruned                 bool
	}{
		// tip below the retention window: nothing is pruned yet
		{0, 5, 100, false},
		{6, 5, 100, false},
		{100, 100, 100, false},
		// tip above the retention window
		{0, 150, 100, true},
		{49, 150, 100, true},
		{50, 150, 100, false},
		{151, 150, 100, false},
	} {
		r.Equal(c.pruned, isHistoryPruned(c.height, c.tip, c.retention), "%+v", c)
	}
}
