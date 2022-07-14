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
	"math/rand"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-antenna-go/v2/account"
	"github.com/iotexproject/iotex-antenna-go/v2/iotex"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

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
		private, err := crypto.GenerateKey()
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
	// keyPairBytes, err := ioutil.ReadFile(keypairsPath)
	// if err != nil {
	// 	return errors.Wrap(err, "failed to read key pairs file")
	// }
	// for Testing
	keypairs := KeyPairs{
		Pairs: []KeyPair{
			{
				PK: "046791faf87db669c1e67e6b338fcb41d02b80336da4ac17a57d83453b4ec439c2fb3c86a25d745dabd37ecc9f5f5ac1371c9f20e8ea0ae1e4feeacf47ffaca469",
				SK: "414efa99dfac6f4095d6954713fb0085268d400d6a05a8ae8a69b5b1c10b4bed",
			},
			{
				PK: "046fb0d32e8a6da2faa91b99ba3bf2dd064d9eb81ba52f864be72064612bc5fc228385ab5cea565a1a709942a4a28f8b70ef5644b903c4ef57fbef41e1746cc910",
				SK: "d1acb5110e20becd3f1e2575e5c67f7befac58cd925767601a5f26223dddd1c8",
			},
			{
				PK: "04b0a3be78f1f30258c8615303d3cdf64faa3aa32e8f9714b16eea614d7c2d9f4824717aebf682d3eb12b4af343fbfab14a351b8f64e59b28a3aa36f9ad57b8983",
				SK: "3aa779c846a62a62217f7481b9c3265f1b7fbc8e3217b7dd192d75a65da8a162",
			},
			{
				PK: "0471165608ad2cfaeea72acc829a9497f1dc08113086846dde8bb31aa3d9a43458df2a9b609f47ecd04f3f951b1acface05c547790cc9702c0864cc8333d1e5464",
				SK: "c9b58691ee786b92980ab1d273254acaa0b31ab49e39e24b809dd6c36a2c165a",
			},
			{
				PK: "04484b6c274699bd0d8d968b773830930528bbcaaa53d13b9aae7146f397efc8b8f6bac4ac751e9332a88f46f10acea24dee68efd1489cae00cf80f1f8baff00e2",
				SK: "9a3296d4237fd5bd2aacc68c09eea1f6b2c225fff46098597889fec8bd703ac1",
			},
		},
	}
	// if err := yaml.Unmarshal(keyPairBytes, &keypairs); err != nil {
	// 	return errors.Wrap(err, "failed to unmarshal key pairs bytes")
	// }

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
		c := iotex.NewAuthedClient(p.api, operatorAccount)
		caller := c.Transfer(recipient, transferValue).SetGasPrice(big.NewInt(rawInjectCfg.transferGasPrice)).SetGasLimit(rawInjectCfg.transferGasLimit)
		if _, err := caller.Call(context.Background()); err != nil {
			log.L().Error("Failed to inject.", zap.Error(err), zap.String("sender", operatorAccount.Address().String()))
		}
		if i != 0 && i%len(addrKeys) == 0 {
			time.Sleep(10 * time.Second)
		}
	}
	time.Sleep(10 * time.Second)
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

func (p *injectProcessor) injectProcessV3(ctx context.Context, actionType int) {
	var (
		bufferSize uint64 = 200
		gaslimit   uint64
		payLoad    string = opMul
		contract   string
	)

	bufferedTxs := make(chan action.SealedEnvelope, bufferSize)

	// query gasPrice
	apiRet2, _ := p.api.SuggestGasPrice(context.Background(), &iotexapi.SuggestGasPriceRequest{})
	gasPrice := new(big.Int).SetUint64(apiRet2.GasPrice)

	// estimate execution gaslimit
	if actionType == actionTypeTransfer {
		gaslimit = 10000
		payLoad = ""
	} else {
		payLoad = opMul
		//deploy contract
		contractGas, err := p.estimateGasLimitForExecution(actionType, action.EmptyAddress, gasPrice, contractByteCode)
		if err != nil {
			panic(err)
		}
		acc := p.accountManager.AccountList[rand.Intn(len(p.accountManager.AccountList))]
		contract, err = util.DeployContract(p.api, acc, p.accountManager.GetAndInc(acc.EncodedAddr), int(contractGas),
			gasPrice.Int64(),
			contractByteCode, int(rawInjectCfg.retryNum), rawInjectCfg.retryInterval)
		if err != nil {
			panic(err)
		}
		gaslimit, err = p.estimateGasLimitForExecution(actionType, contract, gasPrice, payLoad)
		if err != nil {
			panic(err)
		}
	}
	log.L().Info("info", zap.String("contract addr", contract), zap.Uint64("gas limit", gaslimit))
	go p.txGenerate(ctx, bufferedTxs, actionType, gaslimit, gasPrice, payLoad, contract)
	go p.InjectionV3(ctx, bufferedTxs)
}

func (p *injectProcessor) txGenerate(
	ctx context.Context,
	ch chan action.SealedEnvelope,
	actionType int,
	gasLimit uint64,
	gasPrice *big.Int,
	payLoad string,
	contractAddr string,
) {
	for {
		tx, _ := util.ActionGenerator(actionType, p.accountManager, gasLimit, gasPrice, contractAddr, payLoad)
		select {
		case ch <- tx:
			continue
		case <-ctx.Done():
			return
		}
	}

}

func (p *injectProcessor) estimateGasLimitForExecution(actionType int, contractAddr string, gasPrice *big.Int, data string) (uint64, error) {
	var (
		acc = p.accountManager.AccountList[rand.Intn(len(p.accountManager.AccountList))]
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

func (p *injectProcessor) InjectionV3(ctx context.Context, ch chan action.SealedEnvelope) {
	log.L().Info("Initalize the first tx")
	for i := 0; i < len(p.accountManager.AccountList); i++ {
		p.injectV3(<-ch)
		time.Sleep(1 * time.Second)
	}
	time.Sleep(15 * time.Second)

	log.L().Info("Begin inject!")
	ticker := time.NewTicker(time.Duration(time.Second.Nanoseconds() / int64(rawInjectCfg.aps)))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// log.L().Info("buffer", zap.Int("size", len(ch)))
			go p.injectV3(<-ch)
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

func (p *injectProcessor) injectV3(selp action.SealedEnvelope) {

	actHash, _ := selp.Hash()
	log.L().Info("act hash", zap.String("hash", hex.EncodeToString(actHash[:])))
	bo := backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Duration(rawInjectCfg.retryInterval)*time.Second), rawInjectCfg.retryNum)
	rerr := backoff.Retry(func() error {
		_, err := p.api.SendAction(context.Background(), &iotexapi.SendActionRequest{Action: selp.Proto()})
		if err != nil {
			log.L().Error("Failed to inject.", zap.Error(err))
		}
		return err
	}, bo)
	if rerr != nil {
		log.L().Error("Failed to inject.", zap.Error(rerr))
	}
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
		ctx, _ := context.WithTimeout(context.Background(), rawInjectCfg.duration)
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
	configPath       string
	serverAddr       string
	transferGasLimit uint64
	transferGasPrice int64
	transferAmount   int64

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

	randAccounts    int
	loadTokenAmount string
}{}

// var injectCfg = struct {
// 	configPath       string
// 	serverAddr       string
// 	transferGasLimit uint64
// 	transferGasPrice *big.Int
// 	transferAmount   *big.Int

// 	contract          string
// 	executionAmount   *big.Int
// 	executionGasLimit uint64
// 	executionGasPrice *big.Int

// 	actionType      string
// 	retryNum        uint64
// 	retryInterval   time.Duration
// 	duration        time.Duration
// 	resetInterval   time.Duration
// 	aps             int
// 	workers         uint64
// 	checkReceipt    bool
// 	insecure        bool
// 	randAccounts    int
// 	loadTokenAmount *big.Int
// }{}

// func inject(_ []string) {
// 	// transferAmount := big.NewInt(rawInjectCfg.transferAmount)
// 	// transferGasPrice := big.NewInt(rawInjectCfg.transferGasPrice)
// 	// executionGasPrice := big.NewInt(rawInjectCfg.executionGasPrice)
// 	// executionAmount := big.NewInt(rawInjectCfg.executionAmount)
// 	// loadTokenAmount, _ := big.NewInt(0).SetString(rawInjectCfg.loadTokenAmount, 10)

// 	// injectCfg.configPath = rawInjectCfg.configPath
// 	// rawInjectCfg.serverAddr = rawrawInjectCfg.serverAddr
// 	// injectCfg.transferGasLimit = rawInjectCfg.transferGasLimit
// 	// injectCfg.transferGasPrice = transferGasPrice
// 	// injectCfg.transferAmount = transferAmount

// 	// injectCfg.contract = rawInjectCfg.contract
// 	// injectCfg.executionAmount = executionAmount
// 	// injectCfg.executionGasLimit = rawInjectCfg.executionGasLimit
// 	// injectCfg.executionGasPrice = executionGasPrice

// 	// injectCfg.actionType = rawInjectCfg.actionType
// 	// injectCfg.retryNum = rawInjectCfg.retryNum
// 	// injectCfg.retryInterval = rawInjectCfg.retryInterval
// 	// injectCfg.duration = rawInjectCfg.duration
// 	// injectCfg.resetInterval = rawInjectCfg.resetInterval
// 	// injectCfg.aps = rawInjectCfg.aps
// 	// injectCfg.workers = rawInjectCfg.workers
// 	// injectCfg.checkReceipt = rawInjectCfg.checkReceipt
// 	// injectCfg.insecure = rawInjectCfg.insecure
// 	// injectCfg.randAccounts = rawInjectCfg.randAccounts
// 	// injectCfg.loadTokenAmount = loadTokenAmount

// 	return
// }

func init() {
	flag := injectCmd.Flags()
	flag.StringVar(&rawInjectCfg.configPath, "injector-config-path", "./tools/actioninjector.v2/gentsfaddrs.yaml",
		"path of config file of genesis transfer addresses")
	flag.StringVar(&rawInjectCfg.serverAddr, "addr", "api.testnet.iotex.one:443", "target ip:port for grpc connection")
	flag.Int64Var(&rawInjectCfg.transferAmount, "transfer-amount", 0, "execution amount")
	flag.Uint64Var(&rawInjectCfg.transferGasLimit, "transfer-gas-limit", 20000, "transfer gas limit")
	flag.Int64Var(&rawInjectCfg.transferGasPrice, "transfer-gas-price", 1000000000000, "transfer gas price")
	flag.StringVar(&rawInjectCfg.contract, "contract", "io1pmjhyksxmz2xpxn2qmz4gx9qq2kn2gdr8un4xq", "smart contract address")
	flag.Int64Var(&rawInjectCfg.executionAmount, "execution-amount", 0, "execution amount")
	flag.Uint64Var(&rawInjectCfg.executionGasLimit, "execution-gas-limit", 100000, "execution gas limit")
	flag.Int64Var(&rawInjectCfg.executionGasPrice, "execution-gas-price", 1000000000000, "execution gas price")
	flag.StringVar(&rawInjectCfg.actionType, "action-type", "transfer", "action type to inject")
	flag.Uint64Var(&rawInjectCfg.retryNum, "retry-num", 5, "maximum number of rpc retries")
	flag.IntVar(&rawInjectCfg.retryInterval, "retry-interval", 60, "sleep interval between two consecutive rpc retries")
	flag.DurationVar(&rawInjectCfg.duration, "duration", 60*time.Hour, "duration when the injection will run")
	flag.DurationVar(&rawInjectCfg.resetInterval, "reset-interval", 10*time.Second, "time interval to reset nonce counter")
	flag.IntVar(&rawInjectCfg.aps, "aps", 200, "actions to be injected per second")
	flag.IntVar(&rawInjectCfg.randAccounts, "rand-accounts", 20, "number of accounst to use")
	flag.Uint64Var(&rawInjectCfg.workers, "workers", 10, "number of workers")
	flag.BoolVar(&rawInjectCfg.insecure, "insecure", false, "insecure network")
	flag.BoolVar(&rawInjectCfg.checkReceipt, "check-recipt", false, "check recept")
	flag.StringVar(&rawInjectCfg.loadTokenAmount, "load-token-amount", "50000000000000000000", "init load how much token to inject accounts")
	rootCmd.AddCommand(injectCmd)
}
