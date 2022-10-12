// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package disk

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/testutil"
)

func TestMonitor(t *testing.T) {
	require := require.New(t)
	m, err := NewMonitor(t.TempDir(), 30*time.Millisecond)
	require.NoError(err)
	ctx := context.Background()
	err = m.Start(ctx)
	require.NoError(err)
	require.NoError(testutil.WaitUntil(100*time.Millisecond, 1*time.Second, func() (b bool, e error) {
		err = m.Stop(ctx)
		return err == nil, nil
	}))
}

func BenchmarkCheckSpace(b *testing.B) {
	m := &Monitor{path: b.TempDir()}
	// BenchmarkCheckSpace-4   	  522752	      2069 ns/op
	for i := 0; i < b.N; i++ {
		m.checkDiskSpace()
	}
}
