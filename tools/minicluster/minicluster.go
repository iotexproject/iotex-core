// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// usage: make minicluster

package main

import (
	"flag"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/explorer"
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
	// timeout indicates the duration of running nightly build in seconds. Default is 300
	var timeout int
	// aps indicates how many actions to be injected in one second. Default is 0
	var aps float64

	flag.IntVar(&timeout, "timeout", 100, "duration of running nightly build")
	flag.Float64Var(&aps, "aps", 1, "actions to be injected per second")
	flag.Parse()

	// path of config file containing all the public/private key paris of addresses getting transfers
	// from Creator in genesis block
	injectorConfigPath := "./tools/minicluster/gentsfaddrs.yaml"

	chainAddrs, err := util.LoadAddresses(injectorConfigPath, uint32(1))
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load addresses from config path")
	}
	admins := chainAddrs[len(chainAddrs)-numAdmins:]
	delegates := chainAddrs[:len(chainAddrs)-numAdmins]

	// path of config file containing all the transfers and self-nominations in genesis block
	genesisConfigPath := "./tools/minicluster/testnet_actions.yaml"

	// Set mini-cluster configurations
	configs := make([]config.Config, numNodes)
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
		go itx.StartServer(svrs[i], configs[i])
	}

	if err := testutil.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		return svrs[0].ChainService(uint32(1)).Explorer().Port() == 14004, nil
	}); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start explorer JSON-RPC server")
	}

	// target address for jrpc connection. Default is "127.0.0.1:14004"
	jrpcAddr := "127.0.0.1:14004"
	client := explorer.NewExplorerProxy("http://" + jrpcAddr)

	counter, err := util.InitCounter(client, chainAddrs)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize nonce counter")
	}

	// Inject actions to first node
	if aps > 0 {
		// transfer gas limit. Default is 1000000
		transferGasLimit := 1000000
		// transfer gas price. Default is 10
		transferGasPrice := 10
		// transfer payload. Default is ""
		transferPayload := ""
		// vote gas limit. Default is 1000000
		voteGasLimit := 1000000
		// vote gas price. Default is 10
		voteGasPrice := 10
		// smart contract address. Default is "io1qyqsyqcy3kcd2pyfwus69nzgvkwhg8mk8h336dt86pg6cj"
		contract := "io1qyqsyqcy3kcd2pyfwus69nzgvkwhg8mk8h336dt86pg6cj"
		// execution amount. Default is 0
		executionAmount := 0
		// execution gas limit. Default is 1200000
		executionGasLimit := 1200000
		// execution gas price. Default is 10
		executionGasPrice := 10
		// execution data. Default is "2885ad2c"
		executionData := "2885ad2c"
		// maximum number of rpc retries. Default is 5
		retryNum := 5
		// sleeping period between two consecutive rpc retries in seconds. Default is 1
		retryInterval := 1
		// reset interval indicates the interval to reset nonce counter in seconds. Default is 60
		resetInterval := 60
		d := time.Duration(timeout) * time.Second
		wg := &sync.WaitGroup{}
		util.InjectByAps(wg, aps, counter, transferGasLimit, transferGasPrice, transferPayload, voteGasLimit, voteGasPrice,
			contract, executionAmount, executionGasLimit, executionGasPrice, executionData, client, admins, delegates, d,
			retryNum, retryInterval, resetInterval)
		wg.Wait()

		chains := make([]blockchain.Blockchain, numNodes)
		stateHeights := make([]uint64, numNodes)
		bcHeights := make([]uint64, numNodes)
		idealHeight := make([]uint64, numNodes)

		var netTimeout int
		var minTimeout int

		for i := 0; i < numNodes; i++ {
			chains[i] = svrs[i].ChainService(configs[i].Chain.ID).Blockchain()

			stateHeights[i], err = chains[i].GetFactory().Height()
			if err != nil {
				logger.Error().Msg(fmt.Sprintf("Node %d: Can not get State height", i))
			}
			bcHeights[i] = chains[i].TipHeight()
			minTimeout = int(configs[i].Consensus.RollDPoS.Delay/time.Second - configs[i].Consensus.RollDPoS.ProposerInterval/time.Second)
			netTimeout = 0
			if timeout > minTimeout {
				netTimeout = timeout - minTimeout
			}
			idealHeight[i] = uint64((time.Duration(netTimeout) * time.Second) / configs[i].Consensus.RollDPoS.ProposerInterval)

			logger.Info().Msg(fmt.Sprintf("Node#%d blockchain height: %d", i, bcHeights[i]))
			logger.Info().Msg(fmt.Sprintf("Node#%d state      height: %d", i, stateHeights[i]))
			logger.Info().Msg(fmt.Sprintf("Node#%d ideal      height: %d", i, idealHeight[i]))

			if bcHeights[i] != stateHeights[i] {
				logger.Error().Msg(fmt.Sprintf("Node#%d: State height does not match blockchain height", i))
			}
			if math.Abs(float64(bcHeights[i]-idealHeight[i])) > 1 {
				logger.Error().Msg(fmt.Sprintf("blockchain in Node#%d is behind the expected height", i))
			}
		}

		for i := 0; i < numNodes; i++ {
			for j := i + 1; j < numNodes; j++ {
				if math.Abs(float64(bcHeights[i]-bcHeights[j])) > 1 {
					logger.Error().Msg(fmt.Sprintf("blockchain in Node#%d and blockchain in Node#%d are not sync", i, j))
				} else {
					logger.Info().Msg(fmt.Sprintf("blockchain in Node#%d and blockchain in Node#%d are sync", i, j))
				}
			}
		}

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
) config.Config {
	cfg := config.Default

	cfg.NodeType = config.DelegateType

	cfg.Network.Port = networkPort
	cfg.Network.BootstrapNodes = []string{"127.0.0.1:4689"}

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
	cfg.Consensus.RollDPoS.Delay = 10 * time.Second
	cfg.Consensus.RollDPoS.NumSubEpochs = 2
	cfg.Consensus.RollDPoS.EventChanSize = 100000
	cfg.Consensus.RollDPoS.NumDelegates = numNodes
	cfg.Consensus.RollDPoS.TimeBasedRotation = true

	cfg.ActPool.MaxNumActsToPick = 2000

	cfg.System.HTTPMetricsPort = 0

	cfg.Explorer.Enabled = true
	cfg.Explorer.Port = explorerPort

	return cfg
}

func initLogger() {
	l, err := logger.New()
	if err != nil {
		logger.Warn().Err(err).Msg("Cannot config logger, use default one.")
	} else {
		logger.SetLogger(l)
	}
}
