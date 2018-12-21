// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package main

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/explorer"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/iotexproject/iotex-core/tools/util"
)

const (
	testChainPath = "./chain.db"
	testTriePath  = "./trie.db"
)

func TestActionInjector(t *testing.T) {
	require := require.New(t)

	testutil.CleanupPath(t, testChainPath)
	testutil.CleanupPath(t, testTriePath)

	cfg, err := newConfig()
	require.NoError(err)
	ctx := context.Background()
	chainID := cfg.Chain.ID

	// create server
	svr, err := itx.NewServer(cfg)
	require.NoError(err)
	require.Nil(svr.Start(ctx))
	defer func() {
		require.NoError(svr.Stop(ctx))
		testutil.CleanupPath(t, testChainPath)
		testutil.CleanupPath(t, testTriePath)
	}()

	// Create Explorer Client
	client := explorer.NewExplorerProxy(fmt.Sprintf("http://127.0.0.1:%d", svr.ChainService(chainID).Explorer().Port()))

	configPath := "./gentsfaddrs.yaml"

	addrs, err := util.LoadAddresses(configPath, chainID)
	require.NoError(err)

	admins := addrs[len(addrs)-adminNumber:]
	delegates := addrs[:len(addrs)-adminNumber]

	counter, err := util.InitCounter(client, addrs)
	require.NoError(err)

	// Test injectByAps
	aps := float64(50)
	d := time.Second
	resetInterval := 5
	wg := &sync.WaitGroup{}
	retryNum := 5
	retryInterval := 1
	transferGasLimit := 1000000
	transferGasPrice := 10
	transferPayload := ""
	voteGasLimit := 1000000
	voteGasPrice := 10
	contract := "io1qyqsyqcy3kcd2pyfwus69nzgvkwhg8mk8h336dt86pg6cj"
	executionAmount := 0
	executionGasLimit := 1200000
	executionGasPrice := 10
	executionData := "2885ad2c"
	util.InjectByAps(wg, aps, counter, transferGasLimit, transferGasPrice, transferPayload, voteGasLimit, voteGasPrice,
		contract, executionAmount, executionGasLimit, executionGasPrice, executionData, client, admins, delegates, d,
		retryNum, retryInterval, resetInterval)
	wg.Wait()

	// Wait until the injected actions in APS Mode gets into the action pool
	require.NoError(testutil.WaitUntil(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) >= 30, nil
	}))

	acts := svr.ChainService(chainID).ActionPool().PickActs()
	numActsBase := len(acts)

	// Test injectByInterval
	transferNum := 2
	voteNum := 1
	executionNum := 1
	interval := 1
	util.InjectByInterval(transferNum, transferGasLimit, transferGasPrice, transferPayload, voteNum, voteGasLimit,
		voteGasPrice, executionNum, contract, executionAmount, executionGasLimit, executionGasPrice, executionData,
		interval, counter, client, admins, delegates, retryNum, retryInterval)

	// Wait until all the injected actions in Interval Mode gets into the action pool
	err = testutil.WaitUntil(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts)-numActsBase == 4, nil
	})
	require.Nil(err)
}

func newConfig() (config.Config, error) {
	cfg := config.Default
	cfg.NodeType = config.DelegateType
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Chain.ChainDBPath = testChainPath
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Explorer.Enabled = true

	pk, sk, err := crypto.EC283.NewKeyPair()
	if err != nil {
		return config.Config{}, err
	}
	cfg.Chain.ProducerPubKey = keypair.EncodePublicKey(pk)
	cfg.Chain.ProducerPrivKey = keypair.EncodePrivateKey(sk)
	cfg.Network.Port = 0
	cfg.Explorer.Port = 0
	return cfg, nil
}
