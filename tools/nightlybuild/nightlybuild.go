// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// usage: make nightlybuild

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"sync"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/explorer"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/iotexproject/iotex-core/tools/util"
)

const (
	numNodes  = 4
	numAdmins = 2
)

func main() {
	// path of config file containing all the public/private key paris of addresses getting transfers
	// from Creator in genesis block
	var injectorConfigPath string
	// path of config file containing all the transfers and self-nominations in genesis block
	var genesisConfigPath string
	// timeout indicates the duration of running nightly build in seconds. Default is 300
	var timeout int

	// target address for jrpc connection. Default is "127.0.0.1:14004"
	var jrpcAddr string
	// transfer gas limit. Default is 1000000
	var transferGasLimit int
	// transfer gas price. Default is 10
	var transferGasPrice int
	// transfer payload. Default is ""
	var transferPayload string
	// vote gas limit. Default is 1000000
	var voteGasLimit int
	// vote gas price. Default is 10
	var voteGasPrice int
	// smart contract address. Default is "io1qyqsyqcy3kcd2pyfwus69nzgvkwhg8mk8h336dt86pg6cj"
	var contract string
	// execution amount. Default is 0
	var executionAmount int
	// execution gas limit. Default is 1200000
	var executionGasLimit int
	// execution gas price. Default is 10
	var executionGasPrice int
	// execution data. Default is "2885ad2c"
	var executionData string
	// maximum number of rpc retries. Default is 5
	var retryNum int
	// sleeping period between two consecutive rpc retries in seconds. Default is 1
	var retryInterval int
	// aps indicates how many actions to be injected in one second. Default is 0
	var aps int
	// reset interval indicates the interval to reset nonce counter in seconds. Default is 60
	var resetInterval int

	flag.StringVar(&injectorConfigPath, "injector-config-path", "./tools/nightlybuild/gentsfaddrs.yaml",
		"path of config file of genesis transfer addresses")
	flag.StringVar(&genesisConfigPath, "genesis-config-path", "./tools/nightlybuild/testnet_actions.yaml",
		"path of config file of genesis transfers and self-nominations")
	flag.IntVar(&timeout, "timeout", 300, "duration of running nightly build")

	flag.StringVar(&jrpcAddr, "jrpc-addr", "127.0.0.1:14004", "target ip:port for jrpc connection")
	flag.IntVar(&transferGasLimit, "transfer-gas-limit", 1000000, "transfer gas limit")
	flag.IntVar(&transferGasPrice, "transfer-gas-price", 10, "transfer gas price")
	flag.StringVar(&transferPayload, "transfer-payload", "", "transfer payload")
	flag.IntVar(&voteGasLimit, "vote-gas-limit", 1000000, "vote gas limit")
	flag.IntVar(&voteGasPrice, "vote-gas-price", 10, "vote gas price")
	flag.StringVar(&contract, "contract", "io1qyqsyqcy3kcd2pyfwus69nzgvkwhg8mk8h336dt86pg6cj",
		"smart contract address")
	flag.IntVar(&executionAmount, "execution-amount", 50, "execution amount")
	flag.IntVar(&executionGasLimit, "execution-gas-limit", 1200000, "execution gas limit")
	flag.IntVar(&executionGasPrice, "execution-gas-price", 10, "execution gas price")
	flag.StringVar(&executionData, "execution-data", "2885ad2c", "execution data")
	flag.IntVar(&retryNum, "retry-num", 5, "maximum number of rpc retries")
	flag.IntVar(&retryInterval, "retry-interval", 1,
		"sleep interval between two consecutive rpc retries in seconds")
	flag.IntVar(&aps, "aps", 1, "actions to be injected per second")
	flag.IntVar(&resetInterval, "reset-interval", 60,
		"time interval to reset nonce counter in seconds")

	flag.Parse()

	// Load nodes' public/private key pairs
	keyPairBytes, err := ioutil.ReadFile(injectorConfigPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to start injecting actions")
	}
	var keypairs util.KeyPairs
	if err := yaml.Unmarshal(keyPairBytes, &keypairs); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start injecting actions")
	}

	// Construct iotex addresses for loaded nodes
	chainAddrs := make([]*iotxaddress.Address, 0)
	for _, pair := range keypairs.Pairs {
		addr := testutil.ConstructAddress(uint32(1), pair.PK, pair.SK)
		chainAddrs = append(chainAddrs, addr)
	}

	admins := chainAddrs[len(chainAddrs)-numAdmins:]
	delegates := chainAddrs[:len(chainAddrs)-numAdmins]

	// Set mini-cluster configurations
	configs := make([]*config.Config, numNodes)
	for i := 0; i < numNodes; i++ {
		chainDBPath := fmt.Sprintf("./chain%d.db", i+1)
		trieDBPath := fmt.Sprintf("./trie%d.db", i+1)
		networkPort := 4689 + i
		explorerPort := 14004 + i
		config := newConfig(genesisConfigPath, chainDBPath, trieDBPath, chainAddrs[i].PublicKey,
			chainAddrs[i].PrivateKey, networkPort, explorerPort)
		configs[i] = config
	}

	initLogger()

	// Create mini-cluster
	svrs := make([]*itx.Server, numNodes)
	for i := 0; i < numNodes; i++ {
		svr, err := itx.NewServer(configs[i])
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to create server.")
		}
		svrs[i] = svr
	}
	// Start mini-cluster
	for i := 0; i < numNodes; i++ {
		go util.StartNode(svrs[i], configs[i])
	}

	if err := testutil.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		return svrs[0].ChainService(uint32(1)).Explorer().Port() == 14004, nil
	}); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start explorer JSON-RPC server")
	}

	client := explorer.NewExplorerProxy("http://" + jrpcAddr)

	counter := make(map[string]uint64)
	for _, addr := range chainAddrs {
		addrDetails, err := client.GetAddressDetails(addr.RawAddress)
		if err != nil {
			logger.Fatal().Err(err).Str("addr", addr.RawAddress).Msg("Failed to get address details")
		}
		nonce := uint64(addrDetails.PendingNonce)
		counter[addr.RawAddress] = nonce
	}

	rand.Seed(time.Now().UnixNano())

	// Inject actions to first node
	if aps > 0 {
		d := time.Duration(timeout) * time.Second
		wg := &sync.WaitGroup{}
		util.InjectByAps(wg, aps, counter, transferGasLimit, transferGasPrice, transferPayload, voteGasLimit, voteGasPrice,
			contract, executionAmount, executionGasLimit, executionGasPrice, executionData, client, admins, delegates, d,
			retryNum, retryInterval, resetInterval)
		wg.Wait()
	}
}

func newConfig(
	genesisConfigPath,
	chainDBPath,
	trieDBPath string,
	producerPubKey keypair.PublicKey,
	producerPriKey keypair.PrivateKey,
	networkPort,
	explorerPort int,
) *config.Config {
	cfg := config.Default

	cfg.NodeType = config.DelegateType

	cfg.Network.AllowMultiConnsPerHost = true
	cfg.Network.Port = networkPort
	cfg.Network.BootstrapNodes = []string{"127.0.0.1:4689"}
	cfg.Network.NumPeersUpperBound = numNodes
	cfg.Network.NumPeersLowerBound = numNodes
	cfg.Network.TTL = 1

	cfg.Chain.ID = 1
	cfg.Chain.GenesisActionsPath = genesisConfigPath
	cfg.Chain.ChainDBPath = chainDBPath
	cfg.Chain.TrieDBPath = trieDBPath
	cfg.Chain.NumCandidates = numNodes
	cfg.Chain.ProducerPubKey = keypair.EncodePublicKey(producerPubKey)
	cfg.Chain.ProducerPrivKey = keypair.EncodePrivateKey(producerPriKey)

	cfg.Consensus.Scheme = config.RollDPoSScheme
	cfg.Consensus.RollDPoS.DelegateInterval = 10 * time.Second
	cfg.Consensus.RollDPoS.ProposerInterval = 10 * time.Second
	cfg.Consensus.RollDPoS.UnmatchedEventInterval = 4 * time.Second
	cfg.Consensus.RollDPoS.RoundStartTTL = 30 * time.Second
	cfg.Consensus.RollDPoS.AcceptProposeTTL = 4 * time.Second
	cfg.Consensus.RollDPoS.AcceptProposalEndorseTTL = 4 * time.Second
	cfg.Consensus.RollDPoS.AcceptCommitEndorseTTL = 4 * time.Second
	cfg.Consensus.RollDPoS.Delay = 60 * time.Second
	cfg.Consensus.RollDPoS.NumSubEpochs = 2
	cfg.Consensus.RollDPoS.EventChanSize = 100000
	cfg.Consensus.RollDPoS.NumDelegates = numNodes
	cfg.Consensus.RollDPoS.EnableDummyBlock = false
	cfg.Consensus.RollDPoS.TimeBasedRotation = true

	cfg.ActPool.MaxNumActsToPick = 2000

	cfg.System.HTTPMetricsPort = 0

	cfg.Explorer.Enabled = true
	cfg.Explorer.Port = explorerPort

	return &cfg
}

func initLogger() {
	l, err := logger.New()
	if err != nil {
		logger.Warn().Err(err).Msg("Cannot config logger, use default one.")
	} else {
		logger.SetLogger(l)
	}
}
