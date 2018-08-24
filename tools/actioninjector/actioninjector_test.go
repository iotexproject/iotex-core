// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/explorer"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/testutil"
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

	// create server
	svr := itx.NewServer(cfg)
	require.Nil(svr.Start(ctx))
	defer func() {
		require.NoError(svr.Stop(ctx))
		testutil.CleanupPath(t, testChainPath)
		testutil.CleanupPath(t, testTriePath)
	}()

	// Create Explorer Client
	client := explorer.NewExplorerProxy(fmt.Sprintf("http://127.0.0.1:%d", svr.Explorer().Port()))

	configPath := "./gentsfaddrs.yaml"

	// Load Senders' public/private key pairs
	addrBytes, err := ioutil.ReadFile(configPath)
	require.NoError(err)
	addresses := Addresses{}
	err = yaml.Unmarshal(addrBytes, &addresses)
	require.NoError(err)

	// Construct iotex addresses for loaded senders
	addrs := make([]*iotxaddress.Address, 0)
	for _, pkPair := range addresses.PKPairs {
		addr := testutil.ConstructAddress(pkPair.PubKey, pkPair.PriKey)
		addrs = append(addrs, addr)
	}

	// Initiate the map of nonce counter
	counter := make(map[string]uint64)
	for _, addr := range addrs {
		addrDetails, err := client.GetAddressDetails(addr.RawAddress)
		require.NoError(err)
		nonce := uint64(addrDetails.PendingNonce)
		counter[addr.RawAddress] = nonce
	}

	rand.Seed(time.Now().UnixNano())

	// Test injectByAps
	aps := 50
	d := time.Second
	wg := &sync.WaitGroup{}
	retryNum := 5
	retryInterval := 1
	injectByAps(wg, aps, counter, client, addrs, d, make(map[string]bool), retryNum, retryInterval)
	wg.Wait()

	// Wait until the injected actions in APS Mode gets into the action pool
	require.NoError(testutil.WaitUntil(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		transfers, votes, executions := svr.ActionPool().PickActs()
		return len(transfers)+len(votes)+len(executions) >= 30, nil
	}))

	transfers, votes, executions := svr.ActionPool().PickActs()
	numActsBase := len(transfers) + len(votes) + len(executions)

	// Test injectByInterval
	transferNum := 2
	voteNum := 1
	interval := 1
	injectByInterval(transferNum, voteNum, interval, counter, client, addrs, make(map[string]bool), retryNum, retryInterval)

	// Wait until all the injected actions in Interval Mode gets into the action pool
	err = testutil.WaitUntil(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		transfers, votes, executions := svr.ActionPool().PickActs()
		return len(transfers)+len(votes)+len(executions)-numActsBase == 3, nil
	})
	require.Nil(err)
}

func newConfig() (*config.Config, error) {
	cfg := config.Default
	cfg.NodeType = config.DelegateType
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Chain.ChainDBPath = testChainPath
	cfg.Chain.TrieDBPath = testTriePath
	addr, err := iotxaddress.NewAddress(true, iotxaddress.ChainID)
	if err != nil {
		return nil, err
	}
	cfg.Chain.ProducerPubKey = keypair.EncodePublicKey(addr.PublicKey)
	cfg.Chain.ProducerPrivKey = keypair.EncodePrivateKey(addr.PrivateKey)
	cfg.Network.Port = 0
	cfg.Network.PeerMaintainerInterval = 100 * time.Millisecond
	cfg.Explorer.Port = 0
	return &cfg, nil
}
