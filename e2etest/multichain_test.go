// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/iotexproject/iotex-core/test/identityset"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/stretchr/testify/require"
)

func TestTwoChains(t *testing.T) {
	t.Skip()

	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testDBPath := testDBFile.Name()

	cfg := config.Default
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Consensus.Scheme = config.StandaloneScheme
	cfg.Genesis.BlockInterval = time.Second
	cfg.Chain.ProducerPrivKey = identityset.PrivateKey(1).HexString()
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Network.Port = testutil.RandomPort()
	cfg.System.EnableExperimentalActions = true

	svr, err := itx.NewServer(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, svr.Start(ctx))
	defer func() {
		require.NoError(t, svr.Stop(ctx))
	}()

	require.NoError(t, err)

	require.NoError(t, testutil.WaitUntil(time.Second, 20*time.Second, func() (bool, error) {
		return svr.ChainService(2) != nil, nil
	}))

	require.NoError(t, testutil.WaitUntil(time.Second, 20*time.Second, func() (bool, error) {
		return svr.ChainService(2) != nil, nil
	}))

}
