// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/test/util"
)

func TestNetSync(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TestNetSync in short mode.")
	}

	util.CleanupPath(t, testTriePath)
	defer util.CleanupPath(t, testTriePath)
	util.CleanupPath(t, testDBPath)
	defer util.CleanupPath(t, testDBPath)

	cfg := config.Default
	cfg.Network.Addr = "127.0.0.1:4688"
	cfg.Network.BootstrapNodes = []string{"127.0.0.1:4689"}
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.BlockSync.Interval = time.Second

	if testing.Short() {
		t.Skip("Skipping the overlay test in short mode.")
	}

	// create node
	ctx := context.Background()
	svr := itx.NewServer(&cfg)
	require.NotNil(t, svr)
	assert.Nil(t, svr.Start(ctx))

	defer func() {
		require.Nil(t, svr.Stop(ctx))
		util.CleanupPath(t, testTriePath)
		util.CleanupPath(t, testDBPath)
	}()

	select {}
}
