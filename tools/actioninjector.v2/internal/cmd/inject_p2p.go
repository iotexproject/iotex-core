package cmd

import (
	"context"
	"encoding/hex"
	"math/big"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/p2p"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

var injectP2PCmd = &cobra.Command{
	Use:   "injectp2p",
	Short: "Sub-Command for inject actions to IoTeX blockchain via p2p",
	Long:  "Sub-Command for inject actions to IoTeX blockchain via p2p",
	RunE: func(cmd *cobra.Command, args []string) error {
		return injectP2P()
	},
}

type clusterCfg struct {
	chainID        uint32
	bootstraps     []string
	genesisHashHex string
}

var (
	clusterCfgs = map[string]clusterCfg{
		"mainnet": {
			chainID: 1,
			bootstraps: []string{
				"/dns4/bootnode-0.mainnet.iotex.one/tcp/4689/p2p/12D3KooWPfQDF8ASjd4r7jS9e7F1wn6zod7Mf4ERr8etoY6ctQp5",
				"/dns4/bootnode-1.mainnet.iotex.one/tcp/4689/p2p/12D3KooWN4TQ1CWRA7yvJdQCdti1qARLXXu2UEHJfycn3XbnAnRh",
				"/dns4/bootnode-2.mainnet.iotex.one/tcp/4689/p2p/12D3KooWSiktocuUke16bPoW9zrLawEBaEc1UriaPRwm82xbr2BQ",
				"/dns4/bootnode-3.mainnet.iotex.one/tcp/4689/p2p/12D3KooWEsmwaorbZX3HRCnhkMPjMAHzwu3om1pdGrtVm2QaM35n",
				"/dns4/bootnode-4.mainnet.iotex.one/tcp/4689/p2p/12D3KooWHRcgNim4Nau73EEu7aKJZRZPZ21vQ7BE3fG6vENXkduB",
				"/dns4/bootnode-5.mainnet.iotex.one/tcp/4689/p2p/12D3KooWGeHkVDQQFxXpTX1WpPhuuuWYTxPYDUTmaLWWSYx5rmUY",
			},
			genesisHashHex: "b337983730981c2d50f114eed5da9dd20b83c8c5e130beefdb3001dc858cfe8b",
		},
		"testnet": {
			chainID: 2,
			bootstraps: []string{
				"/dns4/bootnode-0.testnet.iotex.one/tcp/4689/ipfs/12D3KooWFnaTYuLo8Mkbm3wzaWHtUuaxBRe24Uiopu15Wr5EhD3o",
				"/dns4/bootnode-1.testnet.iotex.one/tcp/4689/ipfs/12D3KooWS7hkdFozeUqriUxv7zw8Y6NCeV8E5HUbgmVkGJUv4jHS",
			},
			genesisHashHex: "5e31a9f95ca4de82bd6f5ed9b465c6474bba27f1c5d31747393a555ce1f41607",
		},
		"nightly": {
			chainID:        3,
			bootstraps:     []string{"/dns4/bootnode-0.nightly.iotex.one/tcp/4689/p2p/12D3KooWCb1nZdLsR6WBgXqmGnzRvhxASQJaAB3NgwTNk3JE43Wj"},
			genesisHashHex: "b6712a0cd734166b70548b62671295ca549c8fc70ca4036521305793b4f4244e",
		},
	}

	port           = 4689
	injectDuration = time.Minute
	tps            = 10
	concur         = 4
	cluster        = "nightly"
	chainID        uint32
	bootstraps     = []string{}
	genesisHashHex = ""
	accountNum     = 1
	actionNum      = 1
)

func init() {
	injectP2PCmd.Flags().IntVar(&port, "port", port, "port")
	injectP2PCmd.Flags().DurationVar(&injectDuration, "duration", injectDuration, "inject duration")
	injectP2PCmd.Flags().IntVar(&tps, "tps", tps, "transactions per second")
	injectP2PCmd.Flags().IntVar(&concur, "concurrency", concur, "the number of concurrent workers")
	injectP2PCmd.Flags().StringVar(&cluster, "cluster", cluster, "predefined clusters: mainnet, testnet, nightly")
	injectP2PCmd.Flags().Uint32Var(&chainID, "chain-id", chainID, "chain ID if not using predefined cluster")
	injectP2PCmd.Flags().StringSliceVar(&bootstraps, "bootstraps", bootstraps, "bootstrap nodes if not using predefined cluster")
	injectP2PCmd.Flags().StringVar(&genesisHashHex, "genesis-hash", genesisHashHex, "genesis hash if not using predefined cluster")
	injectP2PCmd.Flags().IntVar(&accountNum, "account-num", accountNum, "number of accounts")
	injectP2PCmd.Flags().IntVar(&actionNum, "action-num", actionNum, "number of actions")
	rootCmd.AddCommand(injectP2PCmd)
}

func injectP2P() error {
	cfg := p2p.DefaultConfig
	cfg.Port = port
	cfg.ExternalPort = port
	cfg.ReconnectInterval = 5 * time.Second
	var (
		chainID     uint32
		genesisHash hash.Hash256
	)
	if c, ok := clusterCfgs[cluster]; ok {
		cfg.BootstrapNodes = c.bootstraps
		chainID = c.chainID
		b, err := hex.DecodeString(c.genesisHashHex)
		if err != nil {
			return errors.Wrap(err, "failed to decode genesis hash")
		}
		genesisHash = hash.BytesToHash256(b)
	} else {
		cfg.BootstrapNodes = []string{}
		if len(bootstraps) > 0 {
			cfg.BootstrapNodes = bootstraps
		}
		b, err := hex.DecodeString(genesisHashHex)
		if err != nil {
			return errors.Wrap(err, "failed to decode genesis hash")
		}
		genesisHash = hash.BytesToHash256(b)
	}

	agent := p2p.NewAgent(cfg, chainID, genesisHash, func(ctx context.Context, u uint32, s string, m proto.Message) {}, func(ctx context.Context, u uint32, ai peer.AddrInfo, m proto.Message) {})
	err := agent.Start(context.Background())
	if err != nil {
		return errors.Wrap(err, "failed to start p2p agent")
	}
	log.L().Info("no peers connected, wait for connection...")
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
loop:
	for {
		select {
		case <-ticker.C:
			if peers, err := agent.ConnectedPeers(); err != nil {
				log.L().Error("failed to get connected peers", zap.Error(err))
			} else {
				if len(peers) > 0 {
					log.L().Info("connected to peers", zap.Int("count", len(peers)))
					break loop
				}
				log.L().Info("no peers connected, wait for connection...")
			}
		}
	}

	selps, err := generateTxs()
	if err != nil {
		return errors.Wrap(err, "failed to generate tx")
	}
	log.L().Info("generated actions", zap.Int("count", len(selps)))
	msg := &iotextypes.Actions{
		Actions: make([]*iotextypes.Action, 0, len(selps)),
	}
	for _, selp := range selps {
		msg.Actions = append(msg.Actions, selp.Proto())
	}

	ctx, cancel := context.WithTimeout(context.Background(), injectDuration)
	defer cancel()
	sendTx(ctx, agent, tps, concur, msg)

	return nil
}

func generateTx() (*action.SealedEnvelope, error) {
	nonce := uint64(10)
	receipt := identityset.Address(1)
	e := action.NewEnvelope(action.NewLegacyTx(chainID, nonce, 1000000, big.NewInt(unit.Qev*2)), action.NewTransfer(big.NewInt(1), receipt.String(), nil))
	return action.Sign(e, identityset.PrivateKey(2))
}

func generateTxs() ([]*action.SealedEnvelope, error) {
	var (
		selps     []*action.SealedEnvelope
		accsNonce = make(map[int]uint64)
	)
	for i := 0; i < actionNum; i++ {
		accIdx := i % accountNum
		nonce, ok := accsNonce[accIdx]
		if !ok {
			nonce = 10
		}
		receipt := identityset.Address(rand.Intn(accountNum))
		e := action.NewEnvelope(action.NewLegacyTx(chainID, nonce, 1000000, big.NewInt(unit.Qev*2)), action.NewTransfer(big.NewInt(rand.Int63n(10)), receipt.String(), nil))
		selp, err := action.Sign(e, identityset.PrivateKey(accIdx))
		if err != nil {
			return nil, errors.Wrap(err, "failed to sign action")
		}
		selps = append(selps, selp)
		accsNonce[accIdx] = nonce + 1
		log.L().Info("generated action", zap.Uint64("nonce", nonce), zap.String("sender", selp.SrcPubkey().Address().String()))
	}
	return selps, nil
}

func sendTx(ctx context.Context, agent p2p.Agent, tps, concurrency int, msg proto.Message) {
	log.L().Info("start to inject actions", zap.Int("tps", tps), zap.Int("concurrency", concurrency))
	// Set the desired TPS (transactions per second)
	interval := time.Second * time.Duration(concurrency) / time.Duration(tps)

	wg := sync.WaitGroup{}
	count := uint64(0)
	errCount := uint64(0)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					err := agent.BroadcastOutbound(ctx, msg)
					if err != nil {
						log.L().Error("failed to broadcast outbound", zap.Error(err))
						atomic.AddUint64(&errCount, 1)
					} else {
						log.L().Debug("broadcast outbound success")
					}
					atomic.AddUint64(&count, 1)
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	wg.Add(1)
	go func() {
		// log injection statistics
		defer wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		var lastCount uint64
		var lastErrCount uint64
		for {
			select {
			case <-ticker.C:
				c := atomic.LoadUint64(&count)
				ec := atomic.LoadUint64(&errCount)
				log.L().Info("injected actions",
					zap.Uint64("tps", (c-lastCount)/5),
					zap.Uint64("total", c),
					zap.Uint64("errors", ec-lastErrCount))
				lastCount = c
				lastErrCount = ec
			case <-ctx.Done():
				return
			}
		}
	}()

	// Keep the main goroutine running
	wg.Wait()
}

func printPeers(agent p2p.Agent) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			peers, err := agent.ConnectedPeers()
			if err != nil {
				log.L().Error("failed to get connected peers", zap.Error(err))
				return
			}
			log.L().Info("connected peers", zap.Int("count", len(peers)))
			for _, p := range peers {
				log.L().Info("peer", zap.String("id", p.String()))
			}
		}
	}

}
