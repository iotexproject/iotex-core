package main

import (
	"io/ioutil"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
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

func TestActioninjector(t *testing.T) {
	require := require.New(t)

	testutil.CleanupPath(t, testChainPath)
	defer testutil.CleanupPath(t, testChainPath)
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)

	cfg, err := newConfig()
	require.Nil(err, nil)
	ctx := context.Background()

	// create and start the node
	svr := itx.NewServer(cfg)
	err = svr.Start(ctx)
	require.Nil(err)
	defer svr.Stop(ctx)

	// Start JSON Server
	httpPort := cfg.Explorer.Port
	bcb := func(msg proto.Message) error {
		return svr.P2p().Broadcast(msg)
	}
	explorer.StartJSONServer(svr.Bc(), svr.Cs(), svr.Dp(), svr.Ap(), bcb, false, httpPort, cfg.Explorer.TpsWindow)

	// Create Explorer Client
	client := explorer.NewExplorerProxy("http://127.0.0.1:14004")

	configPath := "./gentsfaddrs.yaml"

	// Load Senders' public/private key pairs
	addrBytes, err := ioutil.ReadFile(configPath)
	require.Nil(err)
	addresses := Addresses{}
	err = yaml.Unmarshal(addrBytes, &addresses)
	require.Nil(err)

	// Construct iotex addresses for loaded senders
	addrs := []*iotxaddress.Address{}
	for _, pkPair := range addresses.PKPairs {
		addr := testutil.ConstructAddress(pkPair.PubKey, pkPair.PriKey)
		addrs = append(addrs, addr)
	}

	// Initiate the map of nonce counter
	counter := make(map[string]uint64)
	for _, addr := range addrs {
		addrDetails, err := client.GetAddressDetails(addr.RawAddress)
		require.Nil(err)
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

	bc := svr.Bc()
	// Wait until committed blocks contain most of the injected actions in APS Mode
	err = testutil.WaitUntil(10*time.Millisecond, time.Second, func() (bool, error) {
		height, _ := bc.TipHeight()
		var tsfCount int
		var voteCount int
		for h := height; h > 0; h-- {
			blk, _ := bc.GetBlockByHeight(h)
			if len(blk.Transfers) > 1 {
				tsfCount += len(blk.Transfers) - 1
			}
			if len(blk.Votes) > 0 {
				voteCount += len(blk.Votes)
			}
		}
		// Excluding coinbase transfers, there should be at least 6 injected actions
		return tsfCount+voteCount >= 6, nil
	})
	require.Nil(err)

	// Test injectByInterval
	transferNum := 2
	voteNum := 1
	interval := 1
	injectByInterval(transferNum, voteNum, interval, counter, client, addrs, make(map[string]bool), retryNum, retryInterval)

	// Wait until committed blocks contain all the injected actions in Interval Mode
	err = testutil.WaitUntil(10*time.Millisecond, 3*time.Second, func() (bool, error) {
		height, _ := bc.TipHeight()
		var tsfCount int
		var voteCount int
		for h := height; h > 0; h-- {
			blk, _ := bc.GetBlockByHeight(h)
			if len(blk.Transfers) > 1 {
				tsfCount += len(blk.Transfers) - 1
			}
			if len(blk.Votes) > 0 {
				voteCount += len(blk.Votes)
			}
		}
		// Excluding coinbase transfers, there should be at least 9 injected actions
		return tsfCount+voteCount >= 9, nil
	})
	require.Nil(err)
}

func newConfig() (*config.Config, error) {
	cfg := config.Default
	cfg.NodeType = config.DelegateType
	cfg.Network.BootstrapNodes = []string{"127.0.0.1:4689", "127.0.0.1:10000"}
	cfg.Consensus.Scheme = config.StandaloneScheme
	cfg.Consensus.BlockCreationInterval = 100 * time.Millisecond
	cfg.Chain.ChainDBPath = testChainPath
	cfg.Chain.TrieDBPath = testTriePath
	addr, err := iotxaddress.NewAddress(true, iotxaddress.ChainID)
	if err != nil {
		return nil, err
	}
	cfg.Chain.ProducerPubKey = keypair.EncodePublicKey(addr.PublicKey)
	cfg.Chain.ProducerPrivKey = keypair.EncodePrivateKey(addr.PrivateKey)
	return &cfg, nil
}
