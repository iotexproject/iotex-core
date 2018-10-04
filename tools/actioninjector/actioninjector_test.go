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
	exp "github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	genesisActionsPath1 = "./testnet_actions_1.yaml"
	testChainPath1      = "./chain1.db"
	testTriePath1       = "./trie1.db"
	genesisActionsPath2 = "./testnet_actions_2.yaml"
	testChainPath2      = "./chain2.db"
	testTriePath2       = "./trie2.db"
)

var (
	chainID1 = uint32(1)
	chainID2 = uint32(2)
)

func TestActionInjector(t *testing.T) {
	require := require.New(t)

	testutil.CleanupPath(t, testChainPath1)
	testutil.CleanupPath(t, testTriePath1)
	testutil.CleanupPath(t, testChainPath2)
	testutil.CleanupPath(t, testTriePath2)

	cfg1, err := newConfig(chainID1, genesisActionsPath1, testChainPath1, testTriePath1)
	require.NoError(err)
	ctx := context.Background()

	// create server with one chain service
	svr, err := itx.NewServer(cfg1)
	require.NoError(err)
	// create another chain service
	cfg2, err := newConfig(chainID2, genesisActionsPath2, testChainPath2, testTriePath2)
	require.NoError(err)

	require.NoError(svr.NewChainService(cfg2))
	require.Nil(svr.Start(ctx))
	defer func() {
		require.NoError(svr.Stop(ctx))
		testutil.CleanupPath(t, testChainPath1)
		testutil.CleanupPath(t, testTriePath1)
		testutil.CleanupPath(t, testChainPath2)
		testutil.CleanupPath(t, testTriePath2)
	}()

	require.NotNil(svr.ChainService(chainID1))
	require.NotNil(svr.ChainService(chainID2))

	// Create two Explorer Clients
	client1 := explorer.NewExplorerProxy(fmt.Sprintf("http://127.0.0.1:%d", svr.ChainService(chainID1).Explorer().Port()))
	client2 := explorer.NewExplorerProxy(fmt.Sprintf("http://127.0.0.1:%d", svr.ChainService(chainID2).Explorer().Port()))
	clientList := []exp.Explorer{client1, client2}
	chainIDList := []uint32{chainID1, chainID2}

	configPath := "./gentsfaddrs_test.yaml"

	// Load Senders' public/private key pairs
	addrBytes, err := ioutil.ReadFile(configPath)
	require.NoError(err)
	addresses := Addresses{}
	err = yaml.Unmarshal(addrBytes, &addresses)
	require.NoError(err)

	// Construct list of iotex addresses for loaded senders, list of explorer clients, and list of chainIDs
	addrsList := make([][]*iotxaddress.Address, 0)
	for i := 0; i < 2; i++ {
		addrs := make([]*iotxaddress.Address, 0)
		for _, pkPair := range addresses.PKPairs {
			addr := testutil.ConstructAddress(chainIDList[i], pkPair.PubKey, pkPair.PriKey)
			addrs = append(addrs, addr)
		}
		addrsList = append(addrsList, addrs)
	}

	// Initiate the list of nonce counter map
	counterList := make([]map[string]uint64, 0)
	for i, addrs := range addrsList {
		counter := make(map[string]uint64)
		for _, addr := range addrs {
			addrDetails, err := clientList[i].GetAddressDetails(addr.RawAddress)
			require.NoError(err)
			nonce := uint64(addrDetails.PendingNonce)
			counter[addr.RawAddress] = nonce
		}
		counterList = append(counterList, counter)
	}

	rand.Seed(time.Now().UnixNano())

	// Test injectByAps
	aps := 50
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
	injectByAps(wg, aps, counterList, transferGasLimit, transferGasPrice, transferPayload, voteGasLimit, voteGasPrice,
		contract, executionAmount, executionGasLimit, executionGasPrice, executionData, clientList, chainIDList, addrsList, d,
		retryNum, retryInterval, resetInterval)
	wg.Wait()

	// Wait until the injected actions in APS Mode gets into the action pool
	require.NoError(testutil.WaitUntil(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		transfers1, votes1, executions1 := svr.ChainService(chainID1).ActionPool().PickActs()
		transfers2, votes2, executions2 := svr.ChainService(chainID2).ActionPool().PickActs()
		return len(transfers1)+len(votes1)+len(executions1) >= 30 && len(transfers2)+len(votes2)+len(executions2) >= 30, nil
	}))

	transfers1, votes1, executions1 := svr.ChainService(chainID1).ActionPool().PickActs()
	numActsBase1 := len(transfers1) + len(votes1) + len(executions1)
	transfers2, votes2, executions2 := svr.ChainService(chainID2).ActionPool().PickActs()
	numActsBase2 := len(transfers2) + len(votes2) + len(executions2)

	// Test injectByInterval
	transferNum := 2
	voteNum := 1
	executionNum := 1
	interval := 1
	injectByInterval(transferNum, transferGasLimit, transferGasPrice, transferPayload, voteNum, voteGasLimit,
		voteGasPrice, executionNum, contract, executionAmount, executionGasLimit, executionGasPrice, executionData,
		interval, counterList, clientList, chainIDList, addrsList, retryNum, retryInterval)

	// Wait until all the injected actions in Interval Mode gets into the action pool
	err = testutil.WaitUntil(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		transfers1, votes1, executions1 := svr.ChainService(chainID1).ActionPool().PickActs()
		transfers2, votes2, executions2 := svr.ChainService(chainID2).ActionPool().PickActs()
		return len(transfers1)+len(votes1)+len(executions1)-numActsBase1 == 4 &&
			len(transfers2)+len(votes2)+len(executions2)-numActsBase2 == 4, nil
	})
	require.Nil(err)
}

func newConfig(chainID uint32, genesisActionsPath, chainDBPath, trieDBPath string) (*config.Config, error) {
	cfg := config.Default
	cfg.NodeType = config.DelegateType
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Chain.GenesisActionsPath = genesisActionsPath
	cfg.Chain.ID = chainID
	cfg.Chain.ChainDBPath = chainDBPath
	cfg.Chain.TrieDBPath = trieDBPath
	var chainIDBytes [4]byte
	enc.MachineEndian.PutUint32(chainIDBytes[:], chainID)
	addr, err := iotxaddress.NewAddress(true, chainIDBytes[:])
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
