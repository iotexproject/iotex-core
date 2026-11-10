// Copyright (c) 2021 IoTeX
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package testutil

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRandomPort(t *testing.T) {
	const count = 512
	ports := make(chan int, count)
	var wg sync.WaitGroup
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ports <- RandomPort()
		}()
	}
	wg.Wait()
	close(ports)

	seen := make(map[int]struct{}, count)
	for port := range ports {
		require.GreaterOrEqual(t, port, minRandomPort)
		require.Less(t, port, maxRandomPort)
		_, duplicate := seen[port]
		require.Falsef(t, duplicate, "port %d was allocated more than once", port)
		seen[port] = struct{}{}
	}
	require.Len(t, seen, count)
}
