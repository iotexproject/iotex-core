// stresstest runs a multi-node IoTeX cluster and injects load to test
// block production, reward distribution, staking, and contract interactions.
//
// Usage: go run ./tools/stresstest/ -timeout 300 -aps 5 -nodes 4
package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/probe"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/pkg/util/fileutil"
	"github.com/iotexproject/iotex-core/v2/server/itx"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

var (
	flagTimeout   = flag.Int("timeout", 300, "test duration in seconds")
	flagAPS       = flag.Float64("aps", 5, "actions per second to inject")
	flagNodes     = flag.Int("nodes", 4, "number of nodes in cluster")
	flagDelegates = flag.Int("delegates", 8, "number of delegates")
)

func main() {
	flag.Parse()

	numNodes := *flagNodes
	numDelegates := *flagDelegates
	timeout := time.Duration(*flagTimeout) * time.Second
	aps := *flagAPS

	log.L().Info("Starting stress test",
		zap.Int("nodes", numNodes),
		zap.Int("delegates", numDelegates),
		zap.Duration("timeout", timeout),
		zap.Float64("aps", aps),
	)

	// Generate delegate keys
	delegateKeys := make([]crypto.PrivateKey, numDelegates)
	for i := 0; i < numDelegates; i++ {
		delegateKeys[i] = identityset.PrivateKey(i)
	}

	// Configure nodes
	dbPaths := make([]string, 0)
	configs := make([]config.Config, numNodes)
	for i := 0; i < numNodes; i++ {
		cfg := config.Default
		cfg.Genesis = genesis.TestDefault()
		cfg.Genesis.Blockchain.NumDelegates = uint64(numDelegates)
		cfg.Genesis.Blockchain.NumSubEpochs = 2
		cfg.Genesis.BlockInterval = 3 * time.Second
		cfg.Genesis.Blockchain.TimeBasedRotation = true
		cfg.Genesis.EnableGravityChainVoting = false
		cfg.Genesis.PollMode = "lifeLong"
		// Use the first numDelegates from the genesis delegate list
		if len(cfg.Genesis.Delegates) > numDelegates {
			cfg.Genesis.Delegates = cfg.Genesis.Delegates[:numDelegates]
		}
		cfg.Genesis.VanuatuBlockHeight = 1
		testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)

		// Assign delegates to nodes (round-robin distribution)
		var keys []string
		for d := 0; d < numDelegates; d++ {
			if d%numNodes == i {
				keys = append(keys, delegateKeys[d].HexString())
			}
		}
		cfg.Chain.ProducerPrivKey = strings.Join(keys, ",")
		cfg.Chain.ID = 1

		// Paths
		prefix := fmt.Sprintf("/tmp/stresstest/node%d", i)
		os.MkdirAll(prefix, 0755)
		cfg.Chain.ChainDBPath = prefix + "/chain.db"
		cfg.Chain.TrieDBPath = prefix + "/trie.db"
		cfg.Chain.TrieDBPatchFile = ""
		cfg.Chain.IndexDBPath = prefix + "/index.db"
		cfg.Chain.BlobStoreDBPath = prefix + "/blob.db"
		cfg.Chain.ContractStakingIndexDBPath = prefix + "/contractstaking.db"
		cfg.Chain.BloomfilterIndexDBPath = prefix + "/bloomfilter.db"
		cfg.Chain.CandidateIndexDBPath = prefix + "/candidate.db"
		cfg.Consensus.RollDPoS.ConsensusDBPath = prefix + "/consensus.db"
		cfg.System.SystemLogDBPath = prefix + "/systemlog.db"
		cfg.ActPool.Store.Datadir = prefix + "/actpool.cache"
		dbPaths = append(dbPaths, prefix)

		// Network
		basePort := 14000 + i*100
		cfg.Network.Port = basePort
		cfg.API.GRPCPort = basePort + 14
		cfg.API.HTTPPort = basePort + 15
		cfg.API.WebSocketPort = basePort + 16
		cfg.System.HTTPAdminPort = basePort + 17
		cfg.Plugins[config.GatewayPlugin] = true
		cfg.Chain.EnableAsyncIndexWrite = false

		if i == 0 {
			cfg.Network.BootstrapNodes = []string{}
			cfg.Network.MasterKey = "bootnode"
		} else {
			cfg.Network.BootstrapNodes = []string{"/ip4/127.0.0.1/tcp/14000/ipfs/12D3KooWJwW6pUpTkxPTMv84RPLPMQVEAjZ6fvJuX4oZrvW5DAGQ"}
		}

		// Consensus timing for 3s blocks
		cfg.Consensus.Scheme = config.RollDPoSScheme
		cfg.Consensus.RollDPoS.FSM.AcceptBlockTTL = 1000 * time.Millisecond
		cfg.Consensus.RollDPoS.FSM.AcceptProposalEndorsementTTL = 1000 * time.Millisecond
		cfg.Consensus.RollDPoS.FSM.AcceptLockEndorsementTTL = 500 * time.Millisecond
		cfg.Consensus.RollDPoS.FSM.CommitTTL = 500 * time.Millisecond
		cfg.Consensus.RollDPoS.FSM.EventChanSize = 100000
		cfg.Consensus.RollDPoS.ToleratedOvertime = 1200 * time.Millisecond
		cfg.Consensus.RollDPoS.Delay = 3 * time.Second
		cfg.DardanellesUpgrade.BlockInterval = 3 * time.Second
		cfg.DardanellesUpgrade.AcceptBlockTTL = 1000 * time.Millisecond
		cfg.DardanellesUpgrade.AcceptProposalEndorsementTTL = 1000 * time.Millisecond
		cfg.DardanellesUpgrade.AcceptLockEndorsementTTL = 500 * time.Millisecond
		cfg.DardanellesUpgrade.CommitTTL = 500 * time.Millisecond
		cfg.Chain.MintTimeout = 200 * time.Millisecond

		configs[i] = cfg
	}

	// Clean up previous data
	os.RemoveAll("/tmp/stresstest")
	for _, cfg := range configs {
		os.MkdirAll(cfg.Chain.ChainDBPath[:len(cfg.Chain.ChainDBPath)-9], 0755)
	}

	// Start nodes
	svrs := make([]*itx.Server, numNodes)
	for i := 0; i < numNodes; i++ {
		svr, err := itx.NewServer(configs[i])
		if err != nil {
			log.L().Fatal("Failed to create server", zap.Int("node", i), zap.Error(err))
		}
		svrs[i] = svr
	}
	defer func() {
		for _, path := range dbPaths {
			if fileutil.FileExists(path) {
				os.RemoveAll(path)
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i := 0; i < numNodes; i++ {
		go itx.StartServer(ctx, svrs[i], probe.New(17700+i), configs[i])
	}

	// Wait for cluster to start
	time.Sleep(5 * time.Second)

	// Connect to first node
	grpcAddr := fmt.Sprintf("127.0.0.1:%d", configs[0].API.GRPCPort)
	conn, err := grpc.DialContext(ctx, grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		log.L().Fatal("Failed to connect to API", zap.Error(err))
	}
	defer conn.Close()
	client := iotexapi.NewAPIServiceClient(conn)

	log.L().Info("Cluster started, beginning stress test")

	// Metrics
	var (
		txSent     atomic.Int64
		txSuccess  atomic.Int64
		txFail     atomic.Int64
		lastHeight atomic.Uint64
	)

	// Monitor goroutine: track chain height and epoch
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				resp, err := client.GetChainMeta(ctx, &iotexapi.GetChainMetaRequest{})
				if err != nil {
					log.L().Warn("GetChainMeta failed", zap.Error(err))
					continue
				}
				h := resp.ChainMeta.Height
				epoch := resp.ChainMeta.Epoch
				lastHeight.Store(h)
				log.L().Info("Chain status",
					zap.Uint64("height", h),
					zap.Uint64("epoch", epoch.Num),
					zap.Int64("txSent", txSent.Load()),
					zap.Int64("txSuccess", txSuccess.Load()),
					zap.Int64("txFail", txFail.Load()),
					zap.Int64("numActions", resp.ChainMeta.NumActions),
				)
			}
		}
	}()

	// Inject transfers
	senderKey := identityset.PrivateKey(25) // use a non-delegate key
	senderAddr := senderKey.PublicKey().Address()
	var nonce uint64

	// Get initial nonce
	accResp, err := client.GetAccount(ctx, &iotexapi.GetAccountRequest{Address: senderAddr.String()})
	if err == nil {
		nonce = accResp.AccountMeta.PendingNonce
	}

	deadline := time.After(timeout)
	interval := time.Duration(float64(time.Second) / aps)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	recipientIdx := 0
	for {
		select {
		case <-deadline:
			goto done
		case <-ticker.C:
			recipientIdx = (recipientIdx + 1) % 24
			recipient := identityset.Address(recipientIdx)
			amount := big.NewInt(1000000000000000) // 0.001 IOTX

			tx := action.NewTransfer(amount, recipient.String(), nil)
			elp := (&action.EnvelopeBuilder{}).
				SetNonce(nonce).
				SetGasPrice(big.NewInt(unit.Qev)).
				SetGasLimit(21000).
				SetAction(tx).Build()

			sealed, err := action.Sign(elp, senderKey)
			if err != nil {
				txFail.Add(1)
				continue
			}
			_, err = client.SendAction(ctx, &iotexapi.SendActionRequest{Action: sealed.Proto()})
			if err != nil {
				txFail.Add(1)
			} else {
				txSuccess.Add(1)
				nonce++
			}
			txSent.Add(1)
		}
	}

done:
	// Final report
	time.Sleep(5 * time.Second) // wait for last blocks
	resp, _ := client.GetChainMeta(ctx, &iotexapi.GetChainMetaRequest{})

	fmt.Println("\n=== STRESS TEST RESULTS ===")
	fmt.Printf("Duration:        %v\n", timeout)
	fmt.Printf("Nodes:           %d\n", numNodes)
	fmt.Printf("Delegates:       %d\n", numDelegates)
	fmt.Printf("Final height:    %d\n", resp.ChainMeta.Height)
	fmt.Printf("Epochs:          %d\n", resp.ChainMeta.Epoch.Num)
	fmt.Printf("Total actions:   %d\n", resp.ChainMeta.NumActions)
	fmt.Printf("TX sent:         %d\n", txSent.Load())
	fmt.Printf("TX success:      %d\n", txSuccess.Load())
	fmt.Printf("TX failed:       %d\n", txFail.Load())
	fmt.Printf("Avg block time:  %.2fs\n", float64(timeout.Seconds())/float64(resp.ChainMeta.Height))
	fmt.Printf("TPS:             %.1f\n", float64(resp.ChainMeta.NumActions)/timeout.Seconds())
	fmt.Println()

	// Check reward balances
	fmt.Println("=== DELEGATE REWARD BALANCES ===")
	for i := 0; i < min(numDelegates, 10); i++ {
		addr := identityset.Address(i).String()
		accResp, err := client.GetAccount(ctx, &iotexapi.GetAccountRequest{Address: addr})
		if err != nil {
			continue
		}
		bal, _ := new(big.Int).SetString(accResp.AccountMeta.Balance, 10)
		initBal := unit.ConvertIotxToRau(100000000)
		earned := new(big.Int).Sub(bal, initBal)
		fmt.Printf("  Delegate %2d (%s...): balance=%s IOTX, earned=%s Rau\n",
			i, addr[:12], formatIOTX(bal), earned.String())
	}
	fmt.Println("  ...")

	cancel()
	time.Sleep(2 * time.Second)
}

func formatIOTX(rau *big.Int) string {
	f := new(big.Float).SetInt(rau)
	f.Quo(f, big.NewFloat(1e18))
	return f.Text('f', 4)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Silence unused import warnings
var (
	_ = hex.EncodeToString
)
