// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// usage: make minicluster

package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/probe"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
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
	// smart contract deployment data. Default is "608060405234801561001057600080fd5b506102f5806100206000396000f3006080604052600436106100615763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416632885ad2c8114610066578063797d9fbd14610070578063cd5e3c5d14610091578063d0e30db0146100b8575b600080fd5b61006e6100c0565b005b61006e73ffffffffffffffffffffffffffffffffffffffff600435166100cb565b34801561009d57600080fd5b506100a6610159565b60408051918252519081900360200190f35b61006e610229565b6100c9336100cb565b565b60006100d5610159565b6040805182815290519192507fbae72e55df73720e0f671f4d20a331df0c0dc31092fda6c573f35ff7f37f283e919081900360200190a160405173ffffffffffffffffffffffffffffffffffffffff8316906305f5e100830280156108fc02916000818181858888f19350505050158015610154573d6000803e3d6000fd5b505050565b604080514460208083019190915260001943014082840152825180830384018152606090920192839052815160009360059361021a9360029391929182918401908083835b602083106101bd5780518252601f19909201916020918201910161019e565b51815160209384036101000a600019018019909216911617905260405191909301945091925050808303816000865af11580156101fe573d6000803e3d6000fd5b5050506040513d602081101561021357600080fd5b5051610261565b81151561022357fe5b06905090565b60408051348152905133917fe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c919081900360200190a2565b600080805b60208110156102c25780600101602060ff160360080260020a848260208110151561028d57fe5b7f010000000000000000000000000000000000000000000000000000000000000091901a810204029190910190600101610266565b50929150505600a165627a7a72305820a426929891673b0a04d7163b60113d28e7d0f48ea667680ba48126c182b872c10029"
	var deployExecData string
	// smart contract interaction data. Default is "d0e30db0"
	var interactExecData string

	flag.IntVar(&timeout, "timeout", 100, "duration of running nightly build")
	flag.Float64Var(&aps, "aps", 1, "actions to be injected per second")
	flag.StringVar(&deployExecData, "deploy-data", "608060405234801561001057600080fd5b506102f5806100206000396000f3006080604052600436106100615763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416632885ad2c8114610066578063797d9fbd14610070578063cd5e3c5d14610091578063d0e30db0146100b8575b600080fd5b61006e6100c0565b005b61006e73ffffffffffffffffffffffffffffffffffffffff600435166100cb565b34801561009d57600080fd5b506100a6610159565b60408051918252519081900360200190f35b61006e610229565b6100c9336100cb565b565b60006100d5610159565b6040805182815290519192507fbae72e55df73720e0f671f4d20a331df0c0dc31092fda6c573f35ff7f37f283e919081900360200190a160405173ffffffffffffffffffffffffffffffffffffffff8316906305f5e100830280156108fc02916000818181858888f19350505050158015610154573d6000803e3d6000fd5b505050565b604080514460208083019190915260001943014082840152825180830384018152606090920192839052815160009360059361021a9360029391929182918401908083835b602083106101bd5780518252601f19909201916020918201910161019e565b51815160209384036101000a600019018019909216911617905260405191909301945091925050808303816000865af11580156101fe573d6000803e3d6000fd5b5050506040513d602081101561021357600080fd5b5051610261565b81151561022357fe5b06905090565b60408051348152905133917fe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c919081900360200190a2565b600080805b60208110156102c25780600101602060ff160360080260020a848260208110151561028d57fe5b7f010000000000000000000000000000000000000000000000000000000000000091901a810204029190910190600101610266565b50929150505600a165627a7a72305820a426929891673b0a04d7163b60113d28e7d0f48ea667680ba48126c182b872c10029",
		"smart contract deployment data")
	flag.StringVar(&interactExecData, "interact-data", "d0e30db0", "smart contract interaction data")
	flag.Parse()

	// path of config file containing all the public/private key paris of addresses getting transfers
	// from Creator in genesis block
	injectorConfigPath := "./tools/minicluster/gentsfaddrs.yaml"

	chainAddrs, err := util.LoadAddresses(injectorConfigPath, uint32(1))
	if err != nil {
		log.L().Fatal("Failed to load addresses from config path", zap.Error(err))
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
		apiPort := 14014 + i
		config := newConfig(genesisConfigPath, chainDBPath, trieDBPath, chainAddrs[i].PriKey,
			networkPort, apiPort)
		if i == 0 {
			config.Network.BootstrapNodes = []string{}
			config.Network.MasterKey = "bootnode"
		}
		configs[i] = config
	}

	// Create mini-cluster
	svrs := make([]*itx.Server, numNodes)
	for i := 0; i < numNodes; i++ {
		svr, err := itx.NewServer(configs[i])
		if err != nil {
			log.L().Fatal("Failed to create server.", zap.Error(err))
		}
		svrs[i] = svr
	}

	// Create and start probe servers
	probeSvrs := make([]*probe.Server, numNodes)
	for i := 0; i < numNodes; i++ {
		probeSvrs[i] = probe.New(7788 + i)
	}
	for i := 0; i < numNodes; i++ {
		err = probeSvrs[i].Start(context.Background())
		if err != nil {
			log.L().Panic("Failed to start probe server")
		}
	}
	// Start mini-cluster
	for i := 0; i < numNodes; i++ {
		go itx.StartServer(context.Background(), svrs[i], probeSvrs[i], configs[i])
	}

	if err := testutil.WaitUntil(10*time.Millisecond, 10*time.Second, func() (bool, error) {
		ret := true
		for i := 0; i < numNodes; i++ {
			resp, err := http.Get("http://localhost:" + strconv.Itoa(7788+i) + "/readiness")
			if err != nil || http.StatusOK != resp.StatusCode {
				ret = false
			}
		}
		return ret, nil
	}); err != nil {
		log.L().Fatal("Failed to start API server", zap.Error(err))
	}
	// target address for grpc connection. Default is "127.0.0.1:14014"
	grpcAddr := "127.0.0.1:14014"
	conn, err := grpc.Dial(grpcAddr, grpc.WithInsecure())
	if err != nil {
		log.L().Error("Failed to connect to API server.")
	}
	client := iotexapi.NewAPIServiceClient(conn)

	counter, err := util.InitCounter(client, chainAddrs)
	if err != nil {
		log.L().Fatal("Failed to initialize nonce counter", zap.Error(err))
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
		// execution amount. Default is 0
		executionAmount := 0
		// execution gas limit. Default is 1200000
		executionGasLimit := 1200000
		// execution gas price. Default is 10
		executionGasPrice := 10
		// maximum number of rpc retries. Default is 5
		retryNum := 5
		// sleeping period between two consecutive rpc retries in seconds. Default is 1
		retryInterval := 1
		// reset interval indicates the interval to reset nonce counter in seconds. Default is 60
		resetInterval := 60
		d := time.Duration(timeout) * time.Second

		// First deploy a smart contract which can be interacted by injected executions
		eHash, err := util.DeployContract(client, counter, delegates, executionGasLimit, executionGasPrice,
			deployExecData, retryNum, retryInterval)
		if err != nil {
			log.L().Fatal("Failed to deploy smart contract", zap.Error(err))
		}
		// Wait until the smart contract is successfully deployed
		var receipt *action.Receipt
		if err := testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
			receipt, err = svrs[0].ChainService(uint32(1)).Blockchain().GetReceiptByActionHash(eHash)
			return receipt != nil, nil
		}); err != nil {
			log.L().Fatal("Failed to get receipt of execution deployment", zap.Error(err))
		}
		contract := receipt.ContractAddress

		wg := &sync.WaitGroup{}
		util.InjectByAps(wg, aps, counter, transferGasLimit, transferGasPrice, transferPayload, voteGasLimit, voteGasPrice,
			contract, executionAmount, executionGasLimit, executionGasPrice, interactExecData, client, admins, delegates, d,
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
				log.S().Errorf("Node %d: Can not get State height", i)
			}
			bcHeights[i] = chains[i].TipHeight()
			minTimeout = int(configs[i].Consensus.RollDPoS.Delay/time.Second - configs[i].Genesis.BlockInterval/time.Second)
			netTimeout = 0
			if timeout > minTimeout {
				netTimeout = timeout - minTimeout
			}
			idealHeight[i] = uint64((time.Duration(netTimeout) * time.Second) / configs[i].Genesis.BlockInterval)

			log.S().Infof("Node#%d blockchain height: %d", i, bcHeights[i])
			log.S().Infof("Node#%d state      height: %d", i, stateHeights[i])
			log.S().Infof("Node#%d ideal      height: %d", i, idealHeight[i])

			if bcHeights[i] != stateHeights[i] {
				log.S().Errorf("Node#%d: State height does not match blockchain height", i)
			}
			if math.Abs(float64(bcHeights[i]-idealHeight[i])) > 1 {
				log.S().Errorf("blockchain in Node#%d is behind the expected height", i)
			}
		}

		for i := 0; i < numNodes; i++ {
			for j := i + 1; j < numNodes; j++ {
				if math.Abs(float64(bcHeights[i]-bcHeights[j])) > 1 {
					log.S().Errorf("blockchain in Node#%d and blockchain in Node#%d are not sync", i, j)
				} else {
					log.S().Infof("blockchain in Node#%d and blockchain in Node#%d are sync", i, j)
				}
			}
		}

	}
}

func newConfig(
	genesisConfigPath,
	chainDBPath,
	trieDBPath string,
	producerPriKey keypair.PrivateKey,
	networkPort,
	apiPort int,
) config.Config {
	cfg := config.Default

	cfg.Network.Port = networkPort
	cfg.Network.BootstrapNodes = []string{"/ip4/127.0.0.1/tcp/4689/ipfs/12D3KooWJwW6pUpTkxPTMv84RPLPMQVEAjZ6fvJuX4oZrvW5DAGQ"}

	cfg.Chain.ID = 1
	cfg.Chain.GenesisActionsPath = genesisConfigPath
	cfg.Chain.ChainDBPath = chainDBPath
	cfg.Chain.TrieDBPath = trieDBPath
	cfg.Chain.NumCandidates = numNodes
	cfg.Chain.EnableIndex = true
	cfg.Chain.EnableAsyncIndexWrite = true

	producerPubKey := &producerPriKey.PublicKey
	cfg.Chain.ProducerPubKey = keypair.EncodePublicKey(producerPubKey)
	cfg.Chain.ProducerPrivKey = keypair.EncodePrivateKey(producerPriKey)

	cfg.Consensus.Scheme = config.RollDPoSScheme
	cfg.Consensus.RollDPoS.FSM.UnmatchedEventInterval = 4 * time.Second
	cfg.Consensus.RollDPoS.FSM.AcceptBlockTTL = 3 * time.Second
	cfg.Consensus.RollDPoS.FSM.AcceptProposalEndorsementTTL = 3 * time.Second
	cfg.Consensus.RollDPoS.FSM.AcceptLockEndorsementTTL = 3 * time.Second
	cfg.Consensus.RollDPoS.FSM.EventChanSize = 100000
	cfg.Consensus.RollDPoS.ToleratedOvertime = 2 * time.Second
	cfg.Consensus.RollDPoS.Delay = 10 * time.Second

	cfg.ActPool.MaxNumActsToPick = 2000

	cfg.System.HTTPMetricsPort = 0

	cfg.API.Enabled = true
	cfg.API.Port = apiPort

	cfg.Genesis.Blockchain.BlockInterval = 10 * time.Second
	cfg.Genesis.Blockchain.NumSubEpochs = 2
	cfg.Genesis.Blockchain.NumDelegates = numNodes
	cfg.Genesis.Blockchain.TimeBasedRotation = true

	return cfg
}
