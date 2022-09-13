// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package fsnotify

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/testutil"
)

func TestWatch(t *testing.T) {
	var (
		require = require.New(t)
		dir     = t.TempDir()
		times   int
	)

	ctx, cancel := context.WithCancel(context.Background())
	go Watch(ctx, dir)

	f, err := os.CreateTemp(dir, "log")
	require.NoError(err)
	defer f.Close()

	require.NoError(testutil.WaitUntil(100*time.Millisecond, 1*time.Second, func() (b bool, e error) {
		err = os.WriteFile(f.Name(), []byte("test"), 0666)
		require.NoError(err)
		times++
		return times >= 5, nil
	}))
	cancel()
}
