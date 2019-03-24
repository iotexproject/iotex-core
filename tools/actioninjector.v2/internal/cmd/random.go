// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	yaml "gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/tools/actioninjector.v2/internal/client"
)

// KeyPairs indicate the keypair of accounts getting transfers from Creator in genesis block
type KeyPairs struct {
	Pairs []KeyPair `yaml:"pkPairs"`
}

// KeyPair contains the public and private key of an address
type KeyPair struct {
	PK string `yaml:"pubKey"`
	SK string `yaml:"priKey"`
}

// AddressKey contains the encoded address and private key of an account
type AddressKey struct {
	EncodedAddr string
	PriKey      keypair.PrivateKey
}

type injectProcessor struct {
	c        *client.Client
	nonces   *sync.Map
	accounts []*AddressKey
}

func newInjectionProcessor() (*injectProcessor, error) {
	c, err := client.New(injectCfg.serverAddr)
	if err != nil {
		return nil, err
	}
	p := &injectProcessor{
		c:      c,
		nonces: &sync.Map{},
	}
	if err := p.loadAccounts(injectCfg.configPath); err != nil {
		return p, err
	}
	p.syncNonces(context.Background())
	return p, nil
}

func (p *injectProcessor) loadAccounts(keypairsPath string) error {
	keyPairBytes, err := ioutil.ReadFile(keypairsPath)
	if err != nil {
		return errors.Wrap(err, "failed to read key pairs file")
	}
	var keypairs KeyPairs
	if err := yaml.Unmarshal(keyPairBytes, &keypairs); err != nil {
		return errors.Wrap(err, "failed to unmarshal key pairs bytes")
	}

	// Construct iotex addresses from loaded key pairs
	addrKeys := make([]*AddressKey, 0)
	for _, pair := range keypairs.Pairs {
		pk, err := keypair.HexStringToPublicKey(pair.PK)
		if err != nil {
			return errors.Wrap(err, "failed to decode public key")
		}
		sk, err := keypair.HexStringToPrivateKey(pair.SK)
		if err != nil {
			return errors.Wrap(err, "failed to decode private key")
		}
		addr, err := address.FromBytes(pk.Hash())
		if err != nil {
			return err
		}
		addrKeys = append(addrKeys, &AddressKey{EncodedAddr: addr.String(), PriKey: sk})
	}
	p.accounts = addrKeys
	return nil
}

func (p *injectProcessor) syncNoncesProcess(ctx context.Context) {
	reset := time.NewTicker(injectCfg.resetInterval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-reset.C:
			p.syncNonces(context.Background())
		}
	}
}

func (p *injectProcessor) syncNonces(ctx context.Context) {
	p.nonces.Range(func(key interface{}, value interface{}) bool {
		addr := key.(string)
		err := backoff.Retry(func() error {
			resp, err := p.c.GetAccount(ctx, addr)
			if err != nil {
				return err
			}
			p.nonces.Store(addr, resp.GetAccountMeta().GetPendingNonce())
			return nil
		}, backoff.NewExponentialBackOff())
		if err != nil {
			log.L().Fatal("Failed to inject actions by APS",
				zap.Error(err),
				zap.String("addr", addr))
		}
		return true
	})
}

func (p *injectProcessor) injectProcess(ctx context.Context) {
	var workers sync.WaitGroup
	ticks := make(chan uint64)
	for i := uint64(0); i < injectCfg.workers; i++ {
		workers.Add(1)
		go p.inject(&workers, ticks)
	}

	defer workers.Wait()
	defer close(ticks)
	interval := uint64(time.Second.Nanoseconds() / int64(injectCfg.aps))
	began, count := time.Now(), uint64(0)
	for {
		now, next := time.Now(), began.Add(time.Duration(count*interval))
		time.Sleep(next.Sub(now))
		select {
		case <-ctx.Done():
			return
		case ticks <- count:
			count++
		default:
			workers.Add(1)
			go p.inject(&workers, ticks)
		}
	}
}

func (p *injectProcessor) inject(workers *sync.WaitGroup, ticks <-chan uint64) {
	defer workers.Done()
	for range ticks {
		selp, err := p.pickAction()
		if err != nil {
			log.L().Error("Failed to create an action", zap.Error(err))
		}
		bo := backoff.WithMaxRetries(backoff.NewConstantBackOff(injectCfg.retryInterval), injectCfg.retryNum)
		if err := backoff.Retry(func() error {
			return p.c.SendAction(context.Background(), selp)
		}, bo); err != nil {
			log.L().Error("Failed to inject.", zap.Error(err))
		}
		log.L().Debug("Sent out the action.")
	}
}

func (p *injectProcessor) pickAction() (action.SealedEnvelope, error) {
	var nonce uint64
	sender := p.accounts[rand.Intn(len(p.accounts))]
	val, ok := p.nonces.Load(sender.EncodedAddr)
	if ok {
		nonce = val.(uint64)
	}
	p.nonces.Store(sender.EncodedAddr, nonce+1)

	bd := &action.EnvelopeBuilder{}
	var elp action.Envelope
	switch rand.Intn(2) {
	case 0:
		amount := int64(0)
		for amount == int64(0) {
			amount = int64(rand.Intn(5))
		}
		recipient := p.accounts[rand.Intn(len(p.accounts))]
		transfer, err := action.NewTransfer(
			nonce, unit.ConvertIotxToRau(amount), recipient.EncodedAddr, injectCfg.transferPayload, injectCfg.transferGasLimit, injectCfg.transferGasPrice)
		if err != nil {
			return action.SealedEnvelope{}, errors.Wrap(err, "failed to create raw transfer")
		}
		elp = bd.SetNonce(nonce).
			SetGasPrice(injectCfg.transferGasPrice).
			SetGasLimit(injectCfg.transferGasLimit).
			SetAction(transfer).Build()
	case 1:
		execution, err := action.NewExecution(injectCfg.contract, nonce, injectCfg.executionAmount, injectCfg.executionGasLimit, injectCfg.executionGasPrice, injectCfg.executionData)
		if err != nil {
			return action.SealedEnvelope{}, errors.Wrap(err, "failed to create raw execution")
		}
		elp = bd.SetNonce(nonce).
			SetGasPrice(injectCfg.executionGasPrice).
			SetGasLimit(injectCfg.executionGasLimit).
			SetAction(execution).Build()
	}

	selp, err := action.Sign(elp, sender.PriKey)
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrapf(err, "failed to sign transfer %v", elp)
	}
	return selp, nil
}

// injectCmd represents the inject command
var injectCmd = &cobra.Command{
	Use:   "random",
	Short: "inject random actions",
	Long:  `inject random actions.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(inject(args))
	},
}

var injectCfg = struct {
	configPath           string
	serverAddr           string
	transferGasLimit     uint64
	rawTransferGasPrice  int64
	transferGasPrice     *big.Int
	rawTransferPayload   string
	transferPayload      []byte
	contract             string
	rawExecutionAmount   int64
	executionAmount      *big.Int
	executionGasLimit    uint64
	rawExecutionGasPrice int64
	executionGasPrice    *big.Int
	rawExecutionData     string
	executionData        []byte
	retryNum             uint64
	retryInterval        time.Duration
	duration             time.Duration
	resetInterval        time.Duration
	aps                  int
	workers              uint64
}{}

func inject(_ []string) string {
	var err error
	injectCfg.transferPayload, err = hex.DecodeString(injectCfg.rawTransferPayload)
	if err != nil {
		return fmt.Sprintf("failed to decode payload %s: %v.", injectCfg.transferPayload, err)
	}
	injectCfg.executionData, err = hex.DecodeString(injectCfg.rawExecutionData)
	if err != nil {
		return fmt.Sprintf("failed to decode data %s: %v", injectCfg.rawExecutionData, err)
	}
	injectCfg.transferGasPrice = big.NewInt(injectCfg.rawTransferGasPrice)
	injectCfg.executionGasPrice = big.NewInt(injectCfg.rawExecutionGasPrice)
	injectCfg.executionAmount = big.NewInt(injectCfg.rawExecutionAmount)
	p, err := newInjectionProcessor()
	if err != nil {
		return fmt.Sprintf("failed to create injector processor: %v.", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), injectCfg.duration)
	defer cancel()
	go p.injectProcess(ctx)
	go p.syncNoncesProcess(ctx)
	<-ctx.Done()
	return ""
}

func init() {
	flag := injectCmd.Flags()
	flag.StringVar(&injectCfg.configPath, "injector-config-path", "./tools/actioninjector.v2/gentsfaddrs.yaml",
		"path of config file of genesis transfer addresses")
	flag.StringVar(&injectCfg.serverAddr, "addr", "127.0.0.1:14004", "target ip:port for grpc connection")
	flag.Uint64Var(&injectCfg.transferGasLimit, "transfer-gas-limit", 20000, "transfer gas limit")
	flag.Int64Var(&injectCfg.rawTransferGasPrice, "transfer-gas-price", unit.Qev, "transfer gas price")
	flag.StringVar(&injectCfg.rawTransferPayload, "transfer-payload", "", "transfer payload")
	flag.StringVar(&injectCfg.contract, "contract", "io1pmjhyksxmz2xpxn2qmz4gx9qq2kn2gdr8un4xq", "smart contract address")
	flag.Int64Var(&injectCfg.rawExecutionAmount, "execution-amount", 50, "execution amount")
	flag.Uint64Var(&injectCfg.executionGasLimit, "execution-gas-limit", 20000, "execution gas limit")
	flag.Int64Var(&injectCfg.rawExecutionGasPrice, "execution-gas-price", unit.Qev, "execution gas price")
	flag.StringVar(&injectCfg.rawExecutionData, "execution-data", "2885ad2c", "execution data")
	flag.Uint64Var(&injectCfg.retryNum, "retry-num", 5, "maximum number of rpc retries")
	flag.DurationVar(&injectCfg.retryInterval, "retry-interval", 1*time.Second, "sleep interval between two consecutive rpc retries")
	flag.DurationVar(&injectCfg.duration, "duration", 60*time.Hour, "duration when the injection will run")
	flag.DurationVar(&injectCfg.resetInterval, "reset-interval", 10*time.Second, "time interval to reset nonce counter")
	flag.IntVar(&injectCfg.aps, "aps", 30, "actions to be injected per second")
	flag.Uint64Var(&injectCfg.workers, "workers", 10, "number of workers")

	rootCmd.AddCommand(injectCmd)
}
