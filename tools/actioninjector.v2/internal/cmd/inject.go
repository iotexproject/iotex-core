// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"math/big"
	rnd "math/rand"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-antenna-go/v2/account"
	"github.com/iotexproject/iotex-antenna-go/v2/iotex"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/tools/util"
)

type (

	// KeyPairs indicate the keypair of accounts getting transfers from Creator in genesis block
	KeyPairs struct {
		Pairs []KeyPair `yaml:"pkPairs"`
	}

	// KeyPair contains the public and private key of an address
	KeyPair struct {
		PK string `yaml:"pubKey"`
		SK string `yaml:"priKey"`
	}

	injectProcessor struct {
		api iotexapi.APIServiceClient
		// nonces         *ttl.Cache
		// accounts       []*util.AddressKey
		accountManager *util.AccountManager
		// tx             []action.SealedEnvelope
		// txIdx          uint64
	}

	txElement struct {
		actionType int
		sender     string
		recepient  string
	}

	feedback struct {
		sender string
		err    error
	}
)

var (
	contractByteCode = "60806040526101f4600055603260015534801561001b57600080fd5b506102558061002b6000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806358931c461461003b5780637f353d5514610045575b600080fd5b61004361004f565b005b61004d610097565b005b60006001905060005b6000548110156100935760028261006f9190610114565b915060028261007e91906100e3565b9150808061008b90610178565b915050610058565b5050565b60005b6001548110156100e057600281908060018154018082558091505060019003906000526020600020016000909190919091505580806100d890610178565b91505061009a565b50565b60006100ee8261016e565b91506100f98361016e565b925082610109576101086101f0565b5b828204905092915050565b600061011f8261016e565b915061012a8361016e565b9250817fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff0483118215151615610163576101626101c1565b5b828202905092915050565b6000819050919050565b60006101838261016e565b91507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff8214156101b6576101b56101c1565b5b600182019050919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601260045260246000fdfea2646970667358221220cb9cada3f1d447c978af17aa3529d6fe4f25f9c5a174085443e371b6940ae99b64736f6c63430008070033"

	opAppend = "7f353d55"
	opMul    = "58931c46"
)

const (
	actionTypeTransfer  = 1
	actionTypeExecution = 2
	actionTypeMixed     = 3
	testnetChainID      = 2
)

func newInjectionProcessor() (*injectProcessor, error) {
	var conn *grpc.ClientConn
	var err error
	grpcctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	log.L().Info("Server Addr", zap.String("endpoint", rawInjectCfg.serverAddr))
	if rawInjectCfg.insecure {
		conn, err = grpc.DialContext(grpcctx, rawInjectCfg.serverAddr, grpc.WithBlock(), grpc.WithInsecure())
	} else {
		conn, err = grpc.DialContext(grpcctx, rawInjectCfg.serverAddr, grpc.WithBlock(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	}
	if err != nil {
		return nil, err
	}
	log.L().Info("server connected")
	api := iotexapi.NewAPIServiceClient(conn)
	// nonceCache, err := ttl.NewCache()
	if err != nil {
		return nil, err
	}
	p := &injectProcessor{
		api: api,
		// nonces: nonceCache,
	}
	p.randAccounts(rawInjectCfg.randAccounts)
	loadValue, _ := new(big.Int).SetString(rawInjectCfg.loadTokenAmount, 10)
	if loadValue.BitLen() != 0 {
		if err := p.loadAccounts(rawInjectCfg.configPath, loadValue); err != nil {
			return p, err
		}
	}
	if err := p.syncNonces(context.Background()); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *injectProcessor) randAccounts(num int) error {
	addrKeys := make([]*util.AddressKey, 0, num)
	for i := 0; i < num; i++ {
		s := hash.Hash256b([]byte{byte(i), byte(100)})
		private, err := crypto.BytesToPrivateKey(s[:])
		if err != nil {
			return err
		}
		a, _ := account.PrivateKeyToAccount(private)
		// p.nonces.Set(a.Address().String(), 1)
		addrKeys = append(addrKeys, &util.AddressKey{PriKey: private, EncodedAddr: a.Address().String()})
	}
	// p.accounts = addrKeys
	p.accountManager = util.NewAccountManager(addrKeys)
	return nil
}

func (p *injectProcessor) loadAccounts(keypairsPath string, transferValue *big.Int) error {
	keyPairBytes, err := os.ReadFile(keypairsPath)
	if err != nil {
		return errors.Wrap(err, "failed to read key pairs file")
	}
	var keypairs KeyPairs
	if err := yaml.Unmarshal(keyPairBytes, &keypairs); err != nil {
		return errors.Wrap(err, "failed to unmarshal key pairs bytes")
	}

	// Construct iotex addresses from loaded key pairs
	var addrKeys []*util.AddressKey
	for _, pair := range keypairs.Pairs {
		pk, err := crypto.HexStringToPublicKey(pair.PK)
		if err != nil {
			return errors.Wrap(err, "failed to decode public key")
		}
		sk, err := crypto.HexStringToPrivateKey(pair.SK)
		if err != nil {
			return errors.Wrap(err, "failed to decode private key")
		}
		addr := pk.Address()
		if addr == nil {
			return errors.New("failed to get address")
		}
		log.L().Info("loaded account", zap.String("addr", addr.String()))
		// p.nonces.Set(addr.String(), 0)
		addrKeys = append(addrKeys, &util.AddressKey{EncodedAddr: addr.String(), PriKey: sk})
	}

	// send tokens
	for i, recipientAddr := range p.accountManager.GetAllAddr() {
		sender := addrKeys[i%len(addrKeys)]
		operatorAccount, _ := account.PrivateKeyToAccount(sender.PriKey)
		recipient, _ := address.FromString(recipientAddr)

		log.L().Info("generated account", zap.String("addr", recipient.String()))
		c := iotex.NewAuthedClient(p.api, rawInjectCfg.chainID, operatorAccount)
		caller := c.Transfer(recipient, transferValue).SetGasPrice(big.NewInt(rawInjectCfg.transferGasPrice)).SetGasLimit(rawInjectCfg.transferGasLimit)
		if _, err := caller.Call(context.Background()); err != nil {
			log.L().Error("Failed to inject.", zap.Error(err), zap.String("sender", operatorAccount.Address().String()))
		}
		if i != 0 && i%len(addrKeys) == 0 {
			time.Sleep(10 * time.Second)
		}
	}
	time.Sleep(time.Second)
	return nil
}

// func (p *injectProcessor) syncNoncesProcess(ctx context.Context) {
// 	reset := time.NewTicker(rawInjectCfg.resetInterval * 3)
// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return
// 		case <-reset.C:
// 			p.syncNonces(context.Background())
// 		}
// 	}
// }

func (p *injectProcessor) syncNonces(ctx context.Context) error {
	for _, addr := range p.accountManager.GetAllAddr() {
		err := backoff.Retry(func() error {
			resp, err := p.api.GetAccount(ctx, &iotexapi.GetAccountRequest{Address: addr})
			if err != nil {
				return err
			}
			p.accountManager.Set(addr, resp.GetAccountMeta().GetPendingNonce())
			return nil
		}, backoff.NewExponentialBackOff())
		if err != nil {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

// func (p *injectProcessor) injectProcess(ctx context.Context) {
// 	var workers sync.WaitGroup
// 	ticks := make(chan uint64)
// 	for i := uint64(0); i < injectCfg.workers; i++ {
// 		workers.Add(1)
// 		go p.inject(&workers, ticks)
// 	}

// 	defer workers.Wait()
// 	defer close(ticks)
// 	interval := uint64(time.Second.Nanoseconds() / int64(injectCfg.aps))
// 	began, count := time.Now(), uint64(0)
// 	for {
// 		now, next := time.Now(), began.Add(time.Duration(count*interval))
// 		time.Sleep(next.Sub(now))
// 		select {
// 		case <-ctx.Done():
// 			return
// 		case ticks <- count:
// 			count++
// 		default:
// 			workers.Add(1)
// 			go p.inject(&workers, ticks)
// 		}
// 	}
// }

// func (p *injectProcessor) injectProcessV2(ctx context.Context) {
// 	var numTx uint64 = 15000
// 	txs, err := util.TxGenerator(numTx, p.api, p.accounts, injectCfg.transferGasLimit, injectCfg.transferGasPrice, 1, "")
// 	if err != nil {
// 		log.L().Error("no act", zap.Error(err))
// 		return
// 	}
// 	p.tx = txs
// 	fmt.Println(len(txs))
// 	for i := 0; i < len(p.accounts); i++ {
// 		p.injectV2()
// 		time.Sleep(1 * time.Second)
// 	}

// 	time.Sleep(15 * time.Second)
// 	log.L().Info("begin inject")
// 	ticker := time.NewTicker(time.Duration(time.Second.Nanoseconds() / int64(injectCfg.aps)))
// 	defer ticker.Stop()
// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return
// 		case <-ticker.C:
// 			// fmt.Println("Tick at")
// 			go p.injectV2()
// 		}
// 	}

// }
func parseHumanSize(s string) int64 {
	s = strings.TrimSpace(s)
	unit := s[len(s)-1:]
	var valueStr string
	if unit == "K" {
		valueStr = s[:len(s)-1]
	} else {
		valueStr = s
	}

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0
	}

	switch strings.ToUpper(unit) {
	case "K":
		return int64(value * 1024)
	default:
		return int64(value)
	}
}

func (p *injectProcessor) injectProcessV3(ctx context.Context, actionType int) {
	var (
		gaslimit    uint64
		payLoad     string = opMul
		contract    string
		bufferedTxs = make(chan action.SealedEnvelope, 1000)
		feedbackCh  = make(chan feedback, 3)
	)

	// query gasPrice
	apiRet2, _ := p.api.SuggestGasPrice(context.Background(), &iotexapi.SuggestGasPriceRequest{})
	gasPrice := new(big.Int).SetUint64(apiRet2.GasPrice)

	// estimate execution gaslimit
	if actionType == actionTypeTransfer {
		gaslimit = 2000000
		gasPrice = big.NewInt(1000000000000)
		payLoad = "646174613a6170706c69636174696f6e2f6a736f6e2c7b2270223a22696f2d3230222c226f70223a226d696e74222c227469636b223a2274657374222c22616d74223a2231303030222c226e6f6e6365223a2231373032383430353336313932227d"
		// if rawInjectCfg.transferPayloadSize != "0" {
		// 	payloadSz := parseHumanSize(rawInjectCfg.transferPayloadSize)
		// 	if payloadSz > 0 {
		// 		randomBytes := make([]byte, payloadSz)
		// 		_, err := rand.Read(randomBytes)
		// 		if err != nil {
		// 			panic(err)
		// 		}
		// 		payLoad = hex.EncodeToString(randomBytes)
		// 	}
		// }
		log.L().Info("transfer meta", zap.Uint64("gasLimit", gaslimit), zap.String("gasPrice", gasPrice.String()), zap.Int("payloadSize", len(payLoad)))
		time.Sleep(3 * time.Second)
	} else {
		payLoad = opMul
		//deploy contract
		contractGas, err := p.estimateGasLimitForExecution(actionType, action.EmptyAddress, gasPrice, contractByteCode)
		if err != nil {
			panic(err)
		}
		acc := p.accountManager.AccountList[rnd.Intn(len(p.accountManager.AccountList))]
		contract, err = util.DeployContract(p.api, acc, p.accountManager.GetAndInc(acc.EncodedAddr), int(contractGas),
			gasPrice.Int64(),
			contractByteCode, int(rawInjectCfg.retryNum), rawInjectCfg.retryInterval, rawInjectCfg.chainID)
		if err != nil {
			panic(err)
		}
		gaslimit, err = p.estimateGasLimitForExecution(actionType, contract, gasPrice, payLoad)
		if err != nil {
			panic(err)
		}
	}
	log.L().Info("info", zap.String("contract addr", contract), zap.Uint64("gas limit", gaslimit))
	go p.txGenerate(ctx, bufferedTxs, feedbackCh, actionType, gaslimit, gasPrice, payLoad, contract)
	go p.InjectionV3(ctx, bufferedTxs, feedbackCh)
	// go p.InjectionV4(ctx, actionType, gaslimit, gasPrice, payLoad, contract)
}

func (p *injectProcessor) txGenerate(
	ctx context.Context,
	ch chan action.SealedEnvelope,
	feedbackCh chan feedback,
	actionType int,
	gasLimit uint64,
	gasPrice *big.Int,
	payLoad string,
	contractAddr string,
) {
	canSend := make(chan bool, 1)
	canSend <- true
	go func() {
		for feedback := range feedbackCh {
			<-canSend
			log.L().Warn("failed to inject, sending willbe delayed.", zap.String("sender", feedback.sender), zap.Error(feedback.err))
			if feedback.err == action.ErrGasLimit {
				time.Sleep(500 * time.Microsecond)
			}
			p.resetAccountNonce(ctx, feedback.sender)
			select {
			case canSend <- true:
			default:
			}
		}
	}()
	for {
		select {
		case <-canSend:
			tx, err := util.ActionGenerator(actionType, p.accountManager, rawInjectCfg.chainID, gasLimit, gasPrice, contractAddr, payLoad)
			if err != nil {
				log.L().Error("no act", zap.Error(err))
				continue
			}
			ch <- tx
			select {
			case canSend <- true:
			default:
			}
		case <-ctx.Done():
			return
		}
	}

}
func (p *injectProcessor) resetAccountNonce(ctx context.Context, addr string) {
	err := backoff.Retry(func() error {
		resp, err := p.api.GetAccount(ctx, &iotexapi.GetAccountRequest{Address: addr})
		if err != nil {
			return err
		}
		p.accountManager.Set(addr, resp.GetAccountMeta().GetPendingNonce())
		return nil
	}, backoff.NewExponentialBackOff())
	if err != nil {
		log.L().Error("Failed to reset nonce.", zap.Error(err), zap.String("addr", addr))
	}
	time.Sleep(100 * time.Millisecond)
}
func (p *injectProcessor) estimateGasLimitForExecution(actionType int, contractAddr string, gasPrice *big.Int, data string) (uint64, error) {
	var (
		acc = p.accountManager.AccountList[rnd.Intn(len(p.accountManager.AccountList))]
	)
	tx, err := action.NewExecution(contractAddr, p.accountManager.Get(acc.EncodedAddr), big.NewInt(0), 0, gasPrice, byteutil.Must(hex.DecodeString(data)))
	if err != nil {
		return 0, err
	}
	gas, err := p.api.EstimateActionGasConsumption(context.Background(), &iotexapi.EstimateActionGasConsumptionRequest{
		Action: &iotexapi.EstimateActionGasConsumptionRequest_Execution{
			Execution: tx.Proto(),
		},
		CallerAddress: acc.EncodedAddr,
	})
	if err != nil {
		return 0, err
	}
	return gas.GetGas(), nil
}

func (p *injectProcessor) InjectionV3(ctx context.Context, ch chan action.SealedEnvelope,
	feedbackCh chan feedback) {
	log.L().Info("Initalize the first tx")
	for i := 0; i < len(p.accountManager.AccountList); i++ {
		p.injectV3(<-ch, feedbackCh)
		time.Sleep(500 * time.Millisecond)
	}
	time.Sleep(time.Second)

	log.L().Info("Begin inject!")
	ticker := time.NewTicker(time.Duration(time.Second.Nanoseconds() / int64(rawInjectCfg.aps)))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// log.L().Info("buffer", zap.Int("size", len(ch)))
			go p.injectV3(<-ch, feedbackCh)
		}
	}
}

// func (p *injectProcessor) inject(workers *sync.WaitGroup, ticks <-chan uint64) {
// 	defer workers.Done()
// 	for range ticks {
// 		go func() {
// 			caller, err := p.pickAction()
// 			if err != nil {
// 				log.L().Error("Failed to create an action", zap.Error(err))
// 			}
// 			var actionHash hash.Hash256
// 			bo := backoff.WithMaxRetries(backoff.NewConstantBackOff(injectCfg.retryInterval), injectCfg.retryNum)
// 			if rerr := backoff.Retry(func() error {
// 				actionHash, err = caller.Call(context.Background())
// 				return err
// 			}, bo); rerr != nil {
// 				log.L().Error("Failed to inject.", zap.Error(rerr))
// 			}

// 			c := iotex.NewReadOnlyClient(p.api)

// 			if injectCfg.checkReceipt {
// 				time.Sleep(25 * time.Second)
// 				var response *iotexapi.GetReceiptByActionResponse
// 				if rerr := backoff.Retry(func() error {
// 					response, err = c.GetReceipt(actionHash).Call(context.Background())
// 					return err
// 				}, bo); rerr != nil {
// 					log.L().Error("Failed to get receipt.", zap.Error(rerr))
// 				}
// 				if response.ReceiptInfo.Receipt.Status != 1 {
// 					log.L().Error("Receipt has failed status.", zap.Uint64("status", response.ReceiptInfo.Receipt.Status))
// 				}
// 			}
// 		}()
// 	}
// }

// func (p *injectProcessor) injectV2() {
// 	selp, err := p.pickActionV2()
// 	if err != nil {
// 		log.L().Error("no act", zap.Error(err))
// 		return
// 	}
// 	actHash, _ := selp.Hash()
// 	log.L().Info("act hash", zap.String("hash", hex.EncodeToString(actHash[:])))
// 	bo := backoff.WithMaxRetries(backoff.NewConstantBackOff(injectCfg.retryInterval), injectCfg.retryNum)
// 	if rerr := backoff.Retry(func() error {
// 		_, err := p.api.SendAction(context.Background(), &iotexapi.SendActionRequest{Action: selp.Proto()})
// 		if err != nil {
// 			log.L().Error("Failed to inject.", zap.Error(err))
// 		}
// 		return err
// 	}, bo); rerr != nil {
// 		log.L().Error("Failed to inject.", zap.Error(rerr))
// 	}
// }

var (
	_injectedActs uint64 = 0
)

func (p *injectProcessor) injectV3(selp action.SealedEnvelope, feedbackCh chan feedback) {
	//actHash, _ := selp.Hash()
	bo := backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Duration(rawInjectCfg.retryInterval)*time.Second), rawInjectCfg.retryNum)
	rerr := backoff.Retry(func() error {
		_, err := p.api.SendAction(context.Background(), &iotexapi.SendActionRequest{Action: selp.Proto()})
		if err != nil {
			if strings.Contains(err.Error(), action.ErrExistedInPool.Error()) {
				return nil
			} else if strings.Contains(err.Error(), action.ErrNonceTooLow.Error()) {
				feedbackCh <- feedback{sender: selp.SrcPubkey().Address().String(), err: action.ErrNonceTooLow}
			} else if strings.Contains(err.Error(), action.ErrNonceTooHigh.Error()) {
				feedbackCh <- feedback{sender: selp.SrcPubkey().Address().String(), err: action.ErrNonceTooHigh}
			} else if strings.Contains(err.Error(), action.ErrGasLimit.Error()) {
				feedbackCh <- feedback{sender: selp.SrcPubkey().Address().String(), err: action.ErrGasLimit}
			}
		}

		return err
	}, bo)
	if rerr != nil {
		log.L().Error("Failed to inject.", zap.Error(rerr))
	}
	atomic.AddUint64(&_injectedActs, 1)
	//log.L().Info("act hash", zap.String("hash", hex.EncodeToString(actHash[:])), zap.Uint64("totalActs", atomic.LoadUint64(&_injectedActs)))
}

// func (p *injectProcessor) pickAction() (iotex.SendActionCaller, error) {
// 	switch injectCfg.actionType {
// 	case "transfer":
// 		return p.transferCaller()
// 	case "execution":
// 		return p.executionCaller()
// 	case "mixed":
// 		if rand.Intn(2) == 0 {
// 			return p.transferCaller()
// 		}
// 		return p.executionCaller()
// 	default:
// 		return p.transferCaller()
// 	}
// }

// func (p *injectProcessor) pickActionV2() (action.SealedEnvelope, error) {
// 	if p.txIdx >= uint64(len(p.tx)) {
// 		return action.SealedEnvelope{}, errors.New("no tx")
// 	}
// 	selp := p.tx[p.txIdx]
// 	atomic.AddUint64(&p.txIdx, 1)
// 	return selp, nil
// }

// func (p *injectProcessor) executionCaller() (iotex.SendActionCaller, error) {
// 	var nonce uint64
// 	sender := p.accounts[rand.Intn(len(p.accounts))]
// 	if val, ok := p.nonces.Get(sender.EncodedAddr); ok {
// 		nonce = val.(uint64)
// 	}
// 	p.nonces.Set(sender.EncodedAddr, nonce+1)

// 	operatorAccount, _ := account.PrivateKeyToAccount(sender.PriKey)
// 	c := iotex.NewAuthedClient(p.api, operatorAccount)
// 	address, _ := address.FromString(injectCfg.contract)
// 	abiJSONVar, _ := abi.JSON(strings.NewReader(_abiStr))
// 	contract := c.Contract(address, abiJSONVar)

// 	data := rand.Int63()
// 	var dataBuf = make([]byte, 8)
// 	binary.BigEndian.PutUint64(dataBuf, uint64(data))
// 	dataHash := sha256.Sum256(dataBuf)

// 	caller := contract.Execute("addHash", uint64(time.Now().Unix()), hex.EncodeToString(dataHash[:])).
// 		SetNonce(nonce).
// 		SetAmount(injectCfg.executionAmount).
// 		SetGasPrice(injectCfg.executionGasPrice).
// 		SetGasLimit(injectCfg.executionGasLimit)

// 	return caller, nil
// }

// func (p *injectProcessor) transferCaller() (iotex.SendActionCaller, error) {
// 	var nonce uint64
// 	sender := p.accounts[rand.Intn(len(p.accounts))]
// 	if val, ok := p.nonces.Get(sender.EncodedAddr); ok {
// 		nonce = val.(uint64)
// 	}
// 	p.nonces.Set(sender.EncodedAddr, nonce+1)

// 	operatorAccount, _ := account.PrivateKeyToAccount(sender.PriKey)
// 	c := iotex.NewAuthedClient(p.api, operatorAccount)

// 	recipient, _ := address.FromString(p.accounts[rand.Intn(len(p.accounts))].EncodedAddr)
// 	data := rand.Int63()
// 	var dataBuf = make([]byte, 8)
// 	binary.BigEndian.PutUint64(dataBuf, uint64(data))
// 	dataHash := sha256.Sum256(dataBuf)
// 	caller := c.Transfer(recipient, injectCfg.transferAmount).
// 		SetPayload(dataHash[:]).
// 		SetNonce(nonce).
// 		SetGasPrice(injectCfg.transferGasPrice).
// 		SetGasLimit(injectCfg.transferGasLimit)
// 	return caller, nil
// }

// injectCmd represents the inject command
var injectCmd = &cobra.Command{
	Use:   "inject",
	Short: "inject actions [options : -m] (default:random)",
	Long:  `inject actions [options : -m] (default:random).`,
	Run: func(cmd *cobra.Command, args []string) {
		p, err := newInjectionProcessor()
		if err != nil {
			panic(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), rawInjectCfg.duration)
		defer cancel()
		var actiontype int
		switch rawInjectCfg.actionType {
		case "transfer":
			actiontype = actionTypeTransfer
		case "execution":
			actiontype = actionTypeExecution
		case "mixed":
			actiontype = actionTypeMixed
		default:
			actiontype = actionTypeTransfer
		}
		go p.injectProcessV3(ctx, actiontype)
		<-ctx.Done()
	},
}

var rawInjectCfg = struct {
	configPath          string
	serverAddr          string
	transferGasLimit    uint64
	transferGasPrice    int64
	transferAmount      int64
	transferPayloadSize string

	contract          string
	executionAmount   int64
	executionGasLimit uint64
	executionGasPrice int64

	actionType    string
	retryNum      uint64
	retryInterval int
	duration      time.Duration
	resetInterval time.Duration
	aps           int
	workers       uint64
	checkReceipt  bool
	insecure      bool
	chainID       uint32

	randAccounts    int
	loadTokenAmount string
}{}

func init() {
	flag := injectCmd.Flags()
	flag.StringVar(&rawInjectCfg.configPath, "injector-config-path", "./tools/actioninjector.v2/gentsfaddrs.yaml",
		"path of config file of genesis transfer addresses")
	// flag.StringVar(&rawInjectCfg.serverAddr, "addr", "ab0ab34e44e114ae5b0ee35da91c8422-1001689351.eu-west-2.elb.amazonaws.com:14014", "target ip:port for grpc connection")
	// flag.StringVar(&rawInjectCfg.serverAddr, "addr", "35.247.25.167:14014", "target ip:port for grpc connection")
	flag.Uint32Var(&rawInjectCfg.chainID, "chain-id", 2, "chain id")
	flag.StringVar(&rawInjectCfg.serverAddr, "addr", "api.testnet.iotex.one:443", "target ip:port for grpc connection")
	flag.Int64Var(&rawInjectCfg.transferAmount, "transfer-amount", 0, "transfer amount")
	flag.Uint64Var(&rawInjectCfg.transferGasLimit, "transfer-gas-limit", 20000, "transfer gas limit")
	flag.Int64Var(&rawInjectCfg.transferGasPrice, "transfer-gas-price", 1000000000000, "transfer gas price")
	flag.StringVar(&rawInjectCfg.transferPayloadSize, "transfer-payload-size", "0", "transfer payload size")
	flag.StringVar(&rawInjectCfg.contract, "contract", "io1pmjhyksxmz2xpxn2qmz4gx9qq2kn2gdr8un4xq", "smart contract address")
	flag.Int64Var(&rawInjectCfg.executionAmount, "execution-amount", 0, "execution amount")
	flag.Uint64Var(&rawInjectCfg.executionGasLimit, "execution-gas-limit", 100000, "execution gas limit")
	flag.Int64Var(&rawInjectCfg.executionGasPrice, "execution-gas-price", 1000000000000, "execution gas price")
	flag.StringVar(&rawInjectCfg.actionType, "action-type", "transfer", "action type to inject")
	flag.Uint64Var(&rawInjectCfg.retryNum, "retry-num", 3, "maximum number of rpc retries")
	flag.IntVar(&rawInjectCfg.retryInterval, "retry-interval", 1, "sleep interval between two consecutive rpc retries")
	flag.DurationVar(&rawInjectCfg.duration, "duration", 10*time.Minute, "duration when the injection will run")
	flag.DurationVar(&rawInjectCfg.resetInterval, "reset-interval", 10*time.Second, "time interval to reset nonce counter")
	flag.IntVar(&rawInjectCfg.aps, "aps", 200, "actions to be injected per second")
	flag.IntVar(&rawInjectCfg.randAccounts, "rand-accounts", 20, "number of accounst to use")
	flag.Uint64Var(&rawInjectCfg.workers, "workers", 10, "number of workers")
	flag.BoolVar(&rawInjectCfg.insecure, "insecure", false, "insecure network")
	flag.BoolVar(&rawInjectCfg.checkReceipt, "check-recipt", false, "check recept")
	flag.StringVar(&rawInjectCfg.loadTokenAmount, "load-token-amount", "5000000000000000000", "init load how much token to inject accounts")
	rootCmd.AddCommand(injectCmd)
}
