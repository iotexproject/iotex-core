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
	m := NewMonitor(30 * time.Millisecond)
	ctx := context.Background()
	err := m.Start(ctx)
	require.NoError(err)
	require.NoError(testutil.WaitUntil(100*time.Millisecond, 1*time.Second, func() (b bool, e error) {
		err = m.Stop(ctx)
		return err == nil, nil
	}))
}

func BenchmarkCheckSpace(b *testing.B) {
	// BenchmarkCheckSpace-4             560012              1849 ns/op             108 B/op          4 allocs/op
	for i := 0; i < b.N; i++ {
		checkDiskSpace()
	}
}
