// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package util

import (
	"context"
	"encoding/hex"
	"io/ioutil"
	"math/big"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/chainservice"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"github.com/iotexproject/iotex-core/tools/executiontester/blockchain"
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

var (
	totalTsfCreated   = uint64(0)
	totalTsfSentToAPI = uint64(0)
	totalTsfSucceeded = uint64(0)
	totalTsfFailed    = uint64(0)
)

//GetTotalTsfCreated returns number of total transfer action created
func GetTotalTsfCreated() uint64 {
	return totalTsfCreated
}

//GetTotalTsfSentToAPI returns number of total transfer action successfully send through GRPC
func GetTotalTsfSentToAPI() uint64 {
	return totalTsfSentToAPI
}

//GetTotalTsfSucceeded returns number of total transfer action created
func GetTotalTsfSucceeded() uint64 {
	return totalTsfSucceeded
}

//GetTotalTsfFailed returns number of total transfer action failed
func GetTotalTsfFailed() uint64 {
	return totalTsfFailed
}

// LoadAddresses loads key pairs from key pair path and construct addresses
func LoadAddresses(keypairsPath string, chainID uint32) ([]*AddressKey, error) {
	// Load Senders' public/private key pairs
	keyPairBytes, err := ioutil.ReadFile(keypairsPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read key pairs file")
	}
	var keypairs KeyPairs
	if err := yaml.Unmarshal(keyPairBytes, &keypairs); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal key pairs bytes")
	}

	// Construct iotex addresses from loaded key pairs
	addrKeys := make([]*AddressKey, 0)
	for _, pair := range keypairs.Pairs {
		pk, err := keypair.HexStringToPublicKey(pair.PK)
		if err != nil {
			return nil, errors.Wrap(err, "failed to decode public key")
		}
		sk, err := keypair.HexStringToPrivateKey(pair.SK)
		if err != nil {
			return nil, errors.Wrap(err, "failed to decode private key")
		}
		addr, err := address.FromBytes(pk.Hash())
		if err != nil {
			return nil, err
		}
		addrKeys = append(addrKeys, &AddressKey{EncodedAddr: addr.String(), PriKey: sk})
	}
	return addrKeys, nil
}

// InitCounter initializes the map of nonce counter of each address
func InitCounter(client iotexapi.APIServiceClient, addrKeys []*AddressKey) (map[string]uint64, error) {
	counter := make(map[string]uint64)
	for _, addrKey := range addrKeys {
		addr := addrKey.EncodedAddr
		err := backoff.Retry(func() error {
			acctDetails, err := client.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: addr})
			if err != nil {
				return err
			}
			counter[addr] = acctDetails.GetAccountMeta().PendingNonce
			return nil
		}, backoff.NewExponentialBackOff())
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get address details of %s", addrKey.EncodedAddr)
		}
	}
	return counter, nil
}

// InjectByAps injects Actions in APS Mode
func InjectByAps(
	wg *sync.WaitGroup,
	aps float64,
	counter map[string]uint64,
	transferGasLimit int,
	transferGasPrice int64,
	transferPayload string,
	voteGasLimit int,
	voteGasPrice int64,
	contract string,
	executionAmount int,
	executionGasLimit int,
	executionGasPrice int64,
	executionData string,
	fpToken blockchain.FpToken,
	fpContract string,
	debtor *AddressKey,
	creditor *AddressKey,
	client iotexapi.APIServiceClient,
	admins []*AddressKey,
	delegates []*AddressKey,
	duration time.Duration,
	retryNum int,
	retryInterval int,
	resetInterval int,
	expectedBalances *map[string]*big.Int,
	cs *chainservice.ChainService,
	pendingActionMap *sync.Map,
) {
	timeout := time.After(duration)
	tick := time.NewTicker(time.Duration(1/aps*1000000) * time.Microsecond)
	reset := time.NewTicker(time.Duration(resetInterval) * time.Second)
	rand.Seed(time.Now().UnixNano())

	randRange := 2
	if fpToken == nil {
		randRange = 1
	}
loop:
	for {
		select {
		case <-timeout:
			break loop
		case <-reset.C:
			for _, admin := range admins {
				addr := admin.EncodedAddr
				err := backoff.Retry(func() error {
					acctDetails, err := client.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: addr})
					if err != nil {
						return err
					}
					counter[addr] = acctDetails.GetAccountMeta().PendingNonce
					return nil
				}, backoff.NewExponentialBackOff())
				if err != nil {
					log.L().Fatal("Failed to inject actions by APS",
						zap.Error(err),
						zap.String("addr", admin.EncodedAddr))
				}
			}
			for _, delegate := range delegates {
				addr := delegate.EncodedAddr
				err := backoff.Retry(func() error {
					acctDetails, err := client.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: addr})
					if err != nil {
						return err
					}
					counter[addr] = acctDetails.GetAccountMeta().PendingNonce
					return nil
				}, backoff.NewExponentialBackOff())
				if err != nil {
					log.L().Fatal("Failed to inject actions by APS",
						zap.Error(err),
						zap.String("addr", delegate.EncodedAddr))
				}
			}
		case <-tick.C:
			wg.Add(1)
			//TODO Currently Vote is skipped because it will fail on balance test and is planned to be removed
			if _, err := CheckPendingActionList(cs,
				pendingActionMap,
				expectedBalances,
			); err != nil {
				log.L().Error(err.Error())
			}
			switch randNum := rand.Intn(randRange); randNum {
			case 0:
				sender, recipient, nonce, amount := createTransferInjection(counter, delegates)
				atomic.AddUint64(&totalTsfCreated, 1)
				go injectTransfer(wg, client, sender, recipient, nonce, amount, uint64(transferGasLimit),
					big.NewInt(transferGasPrice), transferPayload, retryNum, retryInterval, pendingActionMap)
			case 2:
				executor, nonce := createExecutionInjection(counter, delegates)
				go injectExecInteraction(wg, client, executor, contract, nonce, big.NewInt(int64(executionAmount)),
					uint64(executionGasLimit), big.NewInt(executionGasPrice),
					executionData, retryNum, retryInterval, pendingActionMap)
			case 1:
				go injectFpTokenTransfer(wg, fpToken, fpContract, debtor, creditor)
			// vote injection is currently suspended
			case 3:
				sender, recipient, nonce := createVoteInjection(counter, admins, admins)
				go injectVote(wg, client, sender, recipient, nonce, uint64(voteGasLimit),
					big.NewInt(voteGasPrice), retryNum, retryInterval, pendingActionMap)
			}
		}
	}
}

// InjectByInterval injects Actions in Interval Mode
func InjectByInterval(
	transferNum int,
	transferGasLimit int,
	transferGasPrice int,
	transferPayload string,
	voteNum int,
	voteGasLimit int,
	voteGasPrice int,
	executionNum int,
	contract string,
	executionAmount int,
	executionGasLimit int,
	executionGasPrice int,
	executionData string,
	interval int,
	counter map[string]uint64,
	client iotexapi.APIServiceClient,
	admins []*AddressKey,
	delegates []*AddressKey,
	retryNum int,
	retryInterval int,
) {
	rand.Seed(time.Now().UnixNano())
	for transferNum > 0 && voteNum > 0 && executionNum > 0 {
		sender, recipient, nonce, amount := createTransferInjection(counter, delegates)
		injectTransfer(nil, client, sender, recipient, nonce, amount, uint64(transferGasLimit),
			big.NewInt(int64(transferGasPrice)), transferPayload, retryNum, retryInterval, nil)
		time.Sleep(time.Second * time.Duration(interval))

		sender, recipient, nonce = createVoteInjection(counter, admins, delegates)
		injectVote(nil, client, sender, recipient, nonce, uint64(voteGasLimit),
			big.NewInt(int64(voteGasPrice)), retryNum, retryInterval, nil)
		time.Sleep(time.Second * time.Duration(interval))

		executor, nonce := createExecutionInjection(counter, delegates)
		injectExecInteraction(nil, client, executor, contract, nonce, big.NewInt(int64(executionAmount)),
			uint64(executionGasLimit), big.NewInt(int64(executionGasPrice)), executionData, retryNum, retryInterval, nil)
		time.Sleep(time.Second * time.Duration(interval))

		transferNum--
		voteNum--
		executionNum--
	}
	switch {
	case transferNum > 0 && voteNum > 0:
		for transferNum > 0 && voteNum > 0 {
			sender, recipient, nonce, amount := createTransferInjection(counter, delegates)
			injectTransfer(nil, client, sender, recipient, nonce, amount, uint64(transferGasLimit),
				big.NewInt(int64(transferGasPrice)), transferPayload, retryNum, retryInterval, nil)
			time.Sleep(time.Second * time.Duration(interval))

			sender, recipient, nonce = createVoteInjection(counter, admins, delegates)
			injectVote(nil, client, sender, recipient, nonce, uint64(voteGasLimit),
				big.NewInt(int64(voteGasPrice)), retryNum, retryInterval, nil)
			time.Sleep(time.Second * time.Duration(interval))

			transferNum--
			voteNum--
		}
	case transferNum > 0 && executionNum > 0:
		for transferNum > 0 && executionNum > 0 {
			sender, recipient, nonce, amount := createTransferInjection(counter, delegates)
			injectTransfer(nil, client, sender, recipient, nonce, amount, uint64(transferGasLimit),
				big.NewInt(int64(transferGasPrice)), transferPayload, retryNum, retryInterval, nil)
			time.Sleep(time.Second * time.Duration(interval))

			executor, nonce := createExecutionInjection(counter, delegates)
			injectExecInteraction(nil, client, executor, contract, nonce, big.NewInt(int64(executionAmount)),
				uint64(executionGasLimit), big.NewInt(int64(executionGasPrice)), executionData, retryNum, retryInterval, nil)
			time.Sleep(time.Second * time.Duration(interval))

			transferNum--
			executionNum--
		}
	case voteNum > 0 && executionNum > 0:
		for voteNum > 0 && executionNum > 0 {
			sender, recipient, nonce := createVoteInjection(counter, admins, delegates)
			injectVote(nil, client, sender, recipient, nonce, uint64(voteGasLimit),
				big.NewInt(int64(voteGasPrice)), retryNum, retryInterval, nil)
			time.Sleep(time.Second * time.Duration(interval))

			executor, nonce := createExecutionInjection(counter, delegates)
			injectExecInteraction(nil, client, executor, contract, nonce, big.NewInt(int64(executionAmount)),
				uint64(executionGasLimit), big.NewInt(int64(executionGasPrice)), executionData, retryNum, retryInterval, nil)
			time.Sleep(time.Second * time.Duration(interval))

			voteNum--
			executionNum--
		}
	}
	switch {
	case transferNum > 0:
		for transferNum > 0 {
			sender, recipient, nonce, amount := createTransferInjection(counter, delegates)
			injectTransfer(nil, client, sender, recipient, nonce, amount, uint64(transferGasLimit),
				big.NewInt(int64(transferGasPrice)), transferPayload, retryNum, retryInterval, nil)
			time.Sleep(time.Second * time.Duration(interval))
			transferNum--
		}
	case voteNum > 0:
		for voteNum > 0 {
			sender, recipient, nonce := createVoteInjection(counter, admins, delegates)
			injectVote(nil, client, sender, recipient, nonce, uint64(voteGasLimit),
				big.NewInt(int64(voteGasPrice)), retryNum, retryInterval, nil)
			time.Sleep(time.Second * time.Duration(interval))
			voteNum--
		}
	case executionNum > 0:
		for executionNum > 0 {
			executor, nonce := createExecutionInjection(counter, delegates)
			injectExecInteraction(nil, client, executor, contract, nonce, big.NewInt(int64(executionAmount)),
				uint64(executionGasLimit), big.NewInt(int64(executionGasPrice)), executionData, retryNum, retryInterval, nil)
			time.Sleep(time.Second * time.Duration(interval))
			executionNum--
		}
	}
}

// DeployContract deploys a smart contract before starting action injections
func DeployContract(
	client iotexapi.APIServiceClient,
	counter map[string]uint64,
	delegates []*AddressKey,
	executionGasLimit int,
	executionGasPrice int64,
	executionData string,
	retryNum int,
	retryInterval int,
) (hash.Hash256, error) {
	executor, nonce := createExecutionInjection(counter, delegates)
	selp, execution, err := createSignedExecution(executor, action.EmptyAddress, nonce, big.NewInt(0),
		uint64(executionGasLimit), big.NewInt(int64(executionGasPrice)), executionData)
	if err != nil {
		return hash.ZeroHash256, errors.Wrap(err, "failed to create signed execution")
	}
	log.L().Info("Created signed execution")

	injectExecution(selp, execution, client, retryNum, retryInterval)
	return selp.Hash(), nil
}

func injectTransfer(
	wg *sync.WaitGroup,
	c iotexapi.APIServiceClient,
	sender *AddressKey,
	recipient *AddressKey,
	nonce uint64,
	amount int64,
	gasLimit uint64,
	gasPrice *big.Int,
	payload string,
	retryNum int,
	retryInterval int,
	pendingActionMap *sync.Map,
) {
	selp, _, err := createSignedTransfer(sender, recipient, unit.ConvertIotxToRau(amount), nonce, gasLimit,
		gasPrice, payload)
	if err != nil {
		log.L().Fatal("Failed to inject transfer", zap.Error(err))
	}

	log.L().Info("Created signed transfer")

	bo := backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Duration(retryInterval)*time.Second), uint64(retryNum))
	if err := backoff.Retry(func() error {
		_, err := c.SendAction(context.Background(), &iotexapi.SendActionRequest{Action: selp.Proto()})
		return err
	}, bo); err != nil {
		log.L().Error("Failed to inject transfer", zap.Error(err))
	} else if pendingActionMap != nil {
		pendingActionMap.Store(selp.Hash(), 1)
		atomic.AddUint64(&totalTsfSentToAPI, 1)
	}

	if wg != nil {
		wg.Done()
	}
}

func injectVote(
	wg *sync.WaitGroup,
	c iotexapi.APIServiceClient,
	sender *AddressKey,
	recipient *AddressKey,
	nonce uint64,
	gasLimit uint64,
	gasPrice *big.Int,
	retryNum int,
	retryInterval int,
	pendingActionMap *sync.Map,
) {
	selp, _, err := createSignedVote(sender, recipient, nonce, gasLimit, gasPrice)
	if err != nil {
		log.L().Fatal("Failed to inject vote", zap.Error(err))
	}

	log.L().Info("Created signed vote")

	bo := backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Duration(retryInterval)*time.Second), uint64(retryNum))
	if err := backoff.Retry(func() error {
		_, err := c.SendAction(context.Background(), &iotexapi.SendActionRequest{Action: selp.Proto()})
		return err
	}, bo); err != nil {
		log.L().Error("Failed to inject vote", zap.Error(err))
	} else if pendingActionMap != nil {
		pendingActionMap.Store(selp.Hash(), 1)
	}

	if wg != nil {
		wg.Done()
	}
}

func injectExecInteraction(
	wg *sync.WaitGroup,
	c iotexapi.APIServiceClient,
	executor *AddressKey,
	contract string,
	nonce uint64,
	amount *big.Int,
	gasLimit uint64,
	gasPrice *big.Int,
	data string,
	retryNum int,
	retryInterval int,
	pendingActionMap *sync.Map,
) {
	selp, execution, err := createSignedExecution(executor, contract, nonce, amount, gasLimit, gasPrice, data)
	if err != nil {
		log.L().Fatal("Failed to inject execution", zap.Error(err))
	}

	log.L().Info("Created signed execution")

	injectExecution(selp, execution, c, retryNum, retryInterval)

	if pendingActionMap != nil {
		pendingActionMap.Store(selp.Hash(), 1)
	}

	if wg != nil {
		wg.Done()
	}
}

func injectFpTokenTransfer(
	wg *sync.WaitGroup,
	fpToken blockchain.FpToken,
	fpContract string,
	debtor *AddressKey,
	creditor *AddressKey,
) {
	sender := debtor
	recipient := creditor
	balance, err := fpToken.ReadValue(fpContract, "70a08231", debtor.EncodedAddr)
	if err != nil {
		log.L().Error("Failed to read debtor's asset balance", zap.Error(err))
	}
	if balance == int64(0) {
		sender = creditor
		recipient = debtor
		balance, err = fpToken.ReadValue(fpContract, "70a08231", creditor.EncodedAddr)
		if err != nil {
			log.L().Error("Failed to read creditor's asset balance", zap.Error(err))
		}
	}
	transfer := rand.Int63n(balance)
	senderPriKey := sender.PriKey.HexString()
	// Transfer fp token
	if _, err := fpToken.Transfer(fpContract, sender.EncodedAddr, senderPriKey,
		recipient.EncodedAddr, transfer); err != nil {
		log.L().Error("Failed to transfer fp token from debtor to creditor", zap.Error(err))
	}
	if wg != nil {
		wg.Done()
	}
}

// Helper function to get the sender, recipient, nonce, and amount of next injected transfer
func createTransferInjection(
	counter map[string]uint64,
	addrs []*AddressKey,
) (*AddressKey, *AddressKey, uint64, int64) {
	sender := addrs[rand.Intn(len(addrs))]
	recipient := addrs[rand.Intn(len(addrs))]
	nonce := counter[sender.EncodedAddr]
	amount := int64(0)
	for amount == int64(0) {
		amount = int64(rand.Intn(5))
	}
	counter[sender.EncodedAddr]++
	return sender, recipient, nonce, amount
}

// Helper function to get the sender, recipient, and nonce of next injected vote
func createVoteInjection(
	counter map[string]uint64,
	admins []*AddressKey,
	delegates []*AddressKey,
) (*AddressKey, *AddressKey, uint64) {
	sender := admins[rand.Intn(len(admins))]
	recipient := delegates[rand.Intn(len(delegates))]
	nonce := counter[sender.EncodedAddr]
	counter[sender.EncodedAddr]++
	return sender, recipient, nonce
}

// Helper function to get the executor and nonce of next injected execution
func createExecutionInjection(
	counter map[string]uint64,
	addrs []*AddressKey,
) (*AddressKey, uint64) {
	executor := addrs[rand.Intn(len(addrs))]
	nonce := counter[executor.EncodedAddr]
	counter[executor.EncodedAddr]++
	return executor, nonce
}

// Helper function to create and sign a transfer
func createSignedTransfer(
	sender *AddressKey,
	recipient *AddressKey,
	amount *big.Int,
	nonce uint64,
	gasLimit uint64,
	gasPrice *big.Int,
	payload string,
) (action.SealedEnvelope, *action.Transfer, error) {
	transferPayload, err := hex.DecodeString(payload)
	if err != nil {
		return action.SealedEnvelope{}, nil, errors.Wrapf(err, "failed to decode payload %s", payload)
	}
	transfer, err := action.NewTransfer(
		nonce, amount, recipient.EncodedAddr, transferPayload, gasLimit, gasPrice)
	if err != nil {
		return action.SealedEnvelope{}, nil, errors.Wrap(err, "failed to create raw transfer")
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(transfer).Build()
	selp, err := action.Sign(elp, sender.PriKey)
	if err != nil {
		return action.SealedEnvelope{}, nil, errors.Wrapf(err, "failed to sign transfer %v", elp)
	}
	return selp, transfer, nil
}

// Helper function to create and sign a vote
func createSignedVote(
	voter *AddressKey,
	votee *AddressKey,
	nonce uint64,
	gasLimit uint64,
	gasPrice *big.Int,
) (action.SealedEnvelope, *action.Vote, error) {
	vote, err := action.NewVote(nonce, votee.EncodedAddr, gasLimit, gasPrice)
	if err != nil {
		return action.SealedEnvelope{}, nil, errors.Wrap(err, "failed to create raw vote")
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(vote).Build()
	selp, err := action.Sign(elp, voter.PriKey)
	if err != nil {
		return action.SealedEnvelope{}, nil, errors.Wrapf(err, "failed to sign vote %v", elp)
	}
	return selp, vote, nil
}

// Helper function to create and sign an execution
func createSignedExecution(
	executor *AddressKey,
	contract string,
	nonce uint64,
	amount *big.Int,
	gasLimit uint64,
	gasPrice *big.Int,
	data string,
) (action.SealedEnvelope, *action.Execution, error) {
	executionData, err := hex.DecodeString(data)
	if err != nil {
		return action.SealedEnvelope{}, nil, errors.Wrapf(err, "failed to decode data %s", data)
	}
	execution, err := action.NewExecution(contract, nonce, amount, gasLimit, gasPrice, executionData)
	if err != nil {
		return action.SealedEnvelope{}, nil, errors.Wrap(err, "failed to create raw execution")
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(execution).Build()
	selp, err := action.Sign(elp, executor.PriKey)
	if err != nil {
		return action.SealedEnvelope{}, nil, errors.Wrapf(err, "failed to sign execution %v", elp)
	}
	return selp, execution, nil
}

func injectExecution(
	selp action.SealedEnvelope,
	_ *action.Execution,
	c iotexapi.APIServiceClient,
	retryNum int,
	retryInterval int,
) {
	bo := backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Duration(retryInterval)*time.Second), uint64(retryNum))
	if err := backoff.Retry(func() error {
		_, err := c.SendAction(context.Background(), &iotexapi.SendActionRequest{Action: selp.Proto()})
		return err
	}, bo); err != nil {
		log.L().Error("Failed to inject execution", zap.Error(err))
	}
}

//GetAllBalanceMap returns a account balance map of all admins and delegates
func GetAllBalanceMap(
	client iotexapi.APIServiceClient,
	chainaddrs []*AddressKey,
) map[string]*big.Int {
	balanceMap := make(map[string]*big.Int)
	for _, chainaddr := range chainaddrs {
		addr := chainaddr.EncodedAddr
		err := backoff.Retry(func() error {
			acctDetails, err := client.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: addr})
			if err != nil {
				return err
			}
			balanceMap[addr] = big.NewInt(0)
			balanceMap[addr].SetString(acctDetails.GetAccountMeta().Balance, 10)
			return nil
		}, backoff.NewExponentialBackOff())
		if err != nil {
			log.L().Fatal("Failed to Get account balance",
				zap.Error(err),
				zap.String("addr", chainaddr.EncodedAddr))
		}
	}
	return balanceMap
}

// CheckPendingActionList will go through the pending action list, for an executed action:
// 1) update the expectation balance map if the action has been run successfully
// 2) remove the action from pending list
func CheckPendingActionList(
	cs *chainservice.ChainService,
	pendingActionMap *sync.Map,
	balancemap *map[string]*big.Int,
) (bool, error) {
	var retErr error
	empty := true

	pendingActionMap.Range(func(selphash, vi interface{}) bool {
		empty = false
		selp, err := cs.Blockchain().GetActionByActionHash(selphash.(hash.Hash256))
		if err == nil {
			receipt, err := cs.Blockchain().GetReceiptByActionHash(selphash.(hash.Hash256))
			if err != nil {
				retErr = err
				return false
			}
			if receipt.Status == action.SuccessReceiptStatus {

				pbAct := selp.Envelope.Proto()

				switch {
				case pbAct.GetTransfer() != nil:
					act := &action.Transfer{}
					if err := act.LoadProto(pbAct.GetTransfer()); err != nil {
						retErr = err
						return false
					}
					senderaddr, err := address.FromBytes(selp.SrcPubkey().Hash())
					if err != nil {
						retErr = err
						return false
					}

					updateTransferExpectedBalanceMap(balancemap, senderaddr.String(),
						act.Recipient(), act.Amount(), act.Payload(), selp.GasLimit(), selp.GasPrice())
					atomic.AddUint64(&totalTsfSucceeded, 1)

				case pbAct.GetVote() != nil:
					act := &action.Vote{}
					if err := act.LoadProto(pbAct.GetVote()); err != nil {
						retErr = err
						return false
					}
					voteraddr, err := address.FromBytes(selp.SrcPubkey().Hash())
					if err != nil {
						retErr = err
						return false
					}

					updateVoteExpectedBalanceMap(balancemap, voteraddr.String(), selp.GasLimit(), selp.GasPrice())
				case pbAct.GetExecution() != nil:
					act := &action.Execution{}
					if err := act.LoadProto(pbAct.GetExecution()); err != nil {
						retErr = err
						return false
					}
					executoraddr, err := address.FromBytes(selp.SrcPubkey().Hash())
					if err != nil {
						retErr = err
						return false
					}

					updateExecutionExpectedBalanceMap(balancemap, executoraddr.String(), selp.GasLimit(), selp.GasPrice())
				default:
					retErr = errors.New("Unsupported action type for balance check")
					return false
				}
			} else {
				atomic.AddUint64(&totalTsfFailed, 1)
			}
			pendingActionMap.Delete(selphash)
		}
		return true
	})

	return empty, retErr
}
func updateTransferExpectedBalanceMap(
	balancemap *map[string]*big.Int,
	senderAddr string,
	recipientAddr string,
	amount *big.Int,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) {

	gasLimitBig := big.NewInt(int64(gasLimit))

	//calculate gas consumed by payload
	gasUnitPayloadConsumed := new(big.Int).Mul(new(big.Int).SetUint64(action.TransferPayloadGas),
		new(big.Int).SetUint64(uint64(len(payload))))
	gasUnitTransferConsumed := new(big.Int).SetUint64(action.TransferBaseIntrinsicGas)

	//calculate total gas consumed by payload and transfer action
	gasUnitConsumed := new(big.Int).Add(gasUnitPayloadConsumed, gasUnitTransferConsumed)
	if gasLimitBig.Cmp(gasUnitConsumed) < 0 {
		log.L().Fatal("Not enough gas")
	}

	//convert to gas cost
	gasConsumed := new(big.Int).Mul(gasUnitConsumed, gasPrice)

	//total cost of transferred amount, payload, transfer intrinsic
	totalUsed := new(big.Int).Add(gasConsumed, amount)

	//update sender balance
	senderBalance := (*balancemap)[senderAddr]
	if senderBalance.Cmp(totalUsed) < 0 {
		log.L().Fatal("Not enough balance")
	}
	(*balancemap)[senderAddr].Sub(senderBalance, totalUsed)

	//update recipient balance
	recipientBalance := (*balancemap)[recipientAddr]
	(*balancemap)[recipientAddr].Add(recipientBalance, amount)
}

func updateVoteExpectedBalanceMap(
	balancemap *map[string]*big.Int,
	voter string,
	gasLimit uint64,
	gasPrice *big.Int,
) {
	gasLimitBig := new(big.Int).SetUint64(gasLimit)
	gasUnitVoteConsumed := new(big.Int).SetUint64(action.VoteIntrinsicGas)
	if gasLimitBig.Cmp(gasUnitVoteConsumed) < 0 {
		log.L().Fatal("Not enough gas")
	}
	gasConsumed := new(big.Int).Mul(gasUnitVoteConsumed, gasPrice)

	//update sender balance
	senderBalance := (*balancemap)[voter]
	if senderBalance.Cmp(gasConsumed) < 0 {
		log.L().Fatal("Not enough balance")
	}
	(*balancemap)[voter].Sub(senderBalance, gasConsumed)
}

func updateExecutionExpectedBalanceMap(
	balancemap *map[string]*big.Int,
	executor string,
	gasLimit uint64,
	gasPrice *big.Int,
) {
	gasLimitBig := new(big.Int).SetUint64(gasLimit)

	//NOTE: This hard-coded gas comsumption value is precalculted on minicluster deployed test contract only
	gasUnitConsumed := new(big.Int).SetUint64(24028)

	if gasLimitBig.Cmp(gasUnitConsumed) < 0 {
		log.L().Fatal("Not enough gas")
	}
	gasConsumed := new(big.Int).Mul(gasUnitConsumed, gasPrice)

	executorBalance := (*balancemap)[executor]
	if executorBalance.Cmp(gasConsumed) < 0 {
		log.L().Fatal("Not enough balance")
	}
	(*balancemap)[executor].Sub(executorBalance, gasConsumed)

}
