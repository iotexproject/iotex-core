// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package util

import (
	"encoding/hex"
	"io/ioutil"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
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
		pk, err := keypair.DecodePublicKey(pair.PK)
		if err != nil {
			return nil, errors.Wrap(err, "failed to decode public key")
		}
		sk, err := keypair.DecodePrivateKey(pair.SK)
		if err != nil {
			return nil, errors.Wrap(err, "failed to decode private key")
		}
		pkHash := keypair.HashPubKey(pk)
		addr, err := address.FromBytes(pkHash[:])
		if err != nil {
			return nil, err
		}
		addrKeys = append(addrKeys, &AddressKey{EncodedAddr: addr.String(), PriKey: sk})
	}
	return addrKeys, nil
}

// InitCounter initializes the map of nonce counter of each address
func InitCounter(client explorer.Explorer, addrKeys []*AddressKey) (map[string]uint64, error) {
	counter := make(map[string]uint64)
	for _, addrKey := range addrKeys {
		addr := addrKey.EncodedAddr
		err := backoff.Retry(func() error {
			addrDetails, err := client.GetAddressDetails(addr)
			if err != nil {
				return err
			}
			counter[addr] = uint64(addrDetails.PendingNonce)
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
	transferGasPrice int,
	transferPayload string,
	voteGasLimit int,
	voteGasPrice int,
	contract string,
	executionAmount int,
	executionGasLimit int,
	executionGasPrice int,
	executionData string,
	client explorer.Explorer,
	admins []*AddressKey,
	delegates []*AddressKey,
	duration time.Duration,
	retryNum int,
	retryInterval int,
	resetInterval int,
) {
	timeout := time.After(duration)
	tick := time.Tick(time.Duration(1/float64(aps)*1000000) * time.Microsecond)
	reset := time.Tick(time.Duration(resetInterval) * time.Second)
	rand.Seed(time.Now().UnixNano())
loop:
	for {
		select {
		case <-timeout:
			break loop
		case <-reset:
			for _, admin := range admins {
				addr := admin.EncodedAddr
				err := backoff.Retry(func() error {
					addrDetails, err := client.GetAddressDetails(addr)
					if err != nil {
						return err
					}
					counter[addr] = uint64(addrDetails.PendingNonce)
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
					addrDetails, err := client.GetAddressDetails(addr)
					if err != nil {
						return err
					}
					counter[addr] = uint64(addrDetails.PendingNonce)
					return nil
				}, backoff.NewExponentialBackOff())
				if err != nil {
					log.L().Fatal("Failed to inject actions by APS",
						zap.Error(err),
						zap.String("addr", delegate.EncodedAddr))
				}
			}
		case <-tick:
			wg.Add(1)
			switch rand := rand.Intn(3); rand {
			case 0:
				sender, recipient, nonce := createTransferInjection(counter, delegates)
				go injectTransfer(wg, client, sender, recipient, nonce, uint64(transferGasLimit),
					big.NewInt(int64(transferGasPrice)), transferPayload, retryNum, retryInterval)
			case 1:
				sender, recipient, nonce := createVoteInjection(counter, admins, delegates)
				go injectVote(wg, client, sender, recipient, nonce, uint64(voteGasLimit),
					big.NewInt(int64(voteGasPrice)), retryNum, retryInterval)
			case 2:
				executor, nonce := createExecutionInjection(counter, delegates)
				go injectExecInteraction(wg, client, executor, contract, nonce, big.NewInt(int64(executionAmount)),
					uint64(executionGasLimit), big.NewInt(int64(executionGasPrice)),
					executionData, retryNum, retryInterval)
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
	client explorer.Explorer,
	admins []*AddressKey,
	delegates []*AddressKey,
	retryNum int,
	retryInterval int,
) {
	rand.Seed(time.Now().UnixNano())
	for transferNum > 0 && voteNum > 0 && executionNum > 0 {
		sender, recipient, nonce := createTransferInjection(counter, delegates)
		injectTransfer(nil, client, sender, recipient, nonce, uint64(transferGasLimit),
			big.NewInt(int64(transferGasPrice)), transferPayload, retryNum, retryInterval)
		time.Sleep(time.Second * time.Duration(interval))

		sender, recipient, nonce = createVoteInjection(counter, admins, delegates)
		injectVote(nil, client, sender, recipient, nonce, uint64(voteGasLimit),
			big.NewInt(int64(voteGasPrice)), retryNum, retryInterval)
		time.Sleep(time.Second * time.Duration(interval))

		executor, nonce := createExecutionInjection(counter, delegates)
		injectExecInteraction(nil, client, executor, contract, nonce, big.NewInt(int64(executionAmount)),
			uint64(executionGasLimit), big.NewInt(int64(executionGasPrice)), executionData, retryNum, retryInterval)
		time.Sleep(time.Second * time.Duration(interval))

		transferNum--
		voteNum--
		executionNum--
	}
	switch {
	case transferNum > 0 && voteNum > 0:
		for transferNum > 0 && voteNum > 0 {
			sender, recipient, nonce := createTransferInjection(counter, delegates)
			injectTransfer(nil, client, sender, recipient, nonce, uint64(transferGasLimit),
				big.NewInt(int64(transferGasPrice)), transferPayload, retryNum, retryInterval)
			time.Sleep(time.Second * time.Duration(interval))

			sender, recipient, nonce = createVoteInjection(counter, admins, delegates)
			injectVote(nil, client, sender, recipient, nonce, uint64(voteGasLimit),
				big.NewInt(int64(voteGasPrice)), retryNum, retryInterval)
			time.Sleep(time.Second * time.Duration(interval))

			transferNum--
			voteNum--
		}
	case transferNum > 0 && executionNum > 0:
		for transferNum > 0 && executionNum > 0 {
			sender, recipient, nonce := createTransferInjection(counter, delegates)
			injectTransfer(nil, client, sender, recipient, nonce, uint64(transferGasLimit),
				big.NewInt(int64(transferGasPrice)), transferPayload, retryNum, retryInterval)
			time.Sleep(time.Second * time.Duration(interval))

			executor, nonce := createExecutionInjection(counter, delegates)
			injectExecInteraction(nil, client, executor, contract, nonce, big.NewInt(int64(executionAmount)),
				uint64(executionGasLimit), big.NewInt(int64(executionGasPrice)), executionData, retryNum, retryInterval)
			time.Sleep(time.Second * time.Duration(interval))

			transferNum--
			executionNum--
		}
	case voteNum > 0 && executionNum > 0:
		for voteNum > 0 && executionNum > 0 {
			sender, recipient, nonce := createVoteInjection(counter, admins, delegates)
			injectVote(nil, client, sender, recipient, nonce, uint64(voteGasLimit),
				big.NewInt(int64(voteGasPrice)), retryNum, retryInterval)
			time.Sleep(time.Second * time.Duration(interval))

			executor, nonce := createExecutionInjection(counter, delegates)
			injectExecInteraction(nil, client, executor, contract, nonce, big.NewInt(int64(executionAmount)),
				uint64(executionGasLimit), big.NewInt(int64(executionGasPrice)), executionData, retryNum, retryInterval)
			time.Sleep(time.Second * time.Duration(interval))

			voteNum--
			executionNum--
		}
	}
	switch {
	case transferNum > 0:
		for transferNum > 0 {
			sender, recipient, nonce := createTransferInjection(counter, delegates)
			injectTransfer(nil, client, sender, recipient, nonce, uint64(transferGasLimit),
				big.NewInt(int64(transferGasPrice)), transferPayload, retryNum, retryInterval)
			time.Sleep(time.Second * time.Duration(interval))
			transferNum--
		}
	case voteNum > 0:
		for voteNum > 0 {
			sender, recipient, nonce := createVoteInjection(counter, admins, delegates)
			injectVote(nil, client, sender, recipient, nonce, uint64(voteGasLimit),
				big.NewInt(int64(voteGasPrice)), retryNum, retryInterval)
			time.Sleep(time.Second * time.Duration(interval))
			voteNum--
		}
	case executionNum > 0:
		for executionNum > 0 {
			executor, nonce := createExecutionInjection(counter, delegates)
			injectExecInteraction(nil, client, executor, contract, nonce, big.NewInt(int64(executionAmount)),
				uint64(executionGasLimit), big.NewInt(int64(executionGasPrice)), executionData, retryNum, retryInterval)
			time.Sleep(time.Second * time.Duration(interval))
			executionNum--
		}
	}
}

// DeployContract deploys a smart contract before starting action injections
func DeployContract(
	client explorer.Explorer,
	counter map[string]uint64,
	delegates []*AddressKey,
	executionGasLimit int,
	executionGasPrice int,
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
	c explorer.Explorer,
	sender *AddressKey,
	recipient *AddressKey,
	nonce uint64,
	gasLimit uint64,
	gasPrice *big.Int,
	payload string,
	retryNum int,
	retryInterval int,
) {
	amount := int64(0)
	for amount == int64(0) {
		amount = int64(rand.Intn(5))
	}

	selp, tsf, err := createSignedTransfer(sender, recipient, blockchain.ConvertIotxToRau(amount), nonce, gasLimit,
		gasPrice, payload)
	if err != nil {
		log.L().Fatal("Failed to inject transfer", zap.Error(err))
	}

	log.L().Info("Created signed transfer")

	request := explorer.SendTransferRequest{
		Version:      int64(selp.Version()),
		Nonce:        int64(selp.Nonce()),
		Recipient:    selp.DstAddr(),
		SenderPubKey: keypair.EncodePublicKey(selp.SrcPubkey()),
		GasLimit:     int64(selp.GasLimit()),
		Signature:    hex.EncodeToString(selp.Signature()),
		Payload:      hex.EncodeToString(tsf.Payload()),
	}
	if selp.GasPrice() != nil {
		request.GasPrice = selp.GasPrice().String()
	}
	if tsf.Amount() != nil {
		request.Amount = tsf.Amount().String()
	}
	bo := backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Duration(retryInterval)*time.Second), uint64(retryNum))
	if err := backoff.Retry(func() error {
		_, err := c.SendTransfer(request)
		return err
	}, bo); err != nil {
		log.L().Error("Failed to inject transfer", zap.Error(err))
	}
	log.S().Infof("Sent out the signed transfer: %+v", request)

	if wg != nil {
		wg.Done()
	}
}

func injectVote(
	wg *sync.WaitGroup,
	c explorer.Explorer,
	sender *AddressKey,
	recipient *AddressKey,
	nonce uint64,
	gasLimit uint64,
	gasPrice *big.Int,
	retryNum int,
	retryInterval int,
) {
	selp, _, err := createSignedVote(sender, recipient, nonce, gasLimit, gasPrice)
	if err != nil {
		log.L().Fatal("Failed to inject vote", zap.Error(err))
	}

	log.L().Info("Created signed vote")

	request := explorer.SendVoteRequest{
		Version:     int64(selp.Version()),
		Nonce:       int64(selp.Nonce()),
		Votee:       selp.DstAddr(),
		VoterPubKey: keypair.EncodePublicKey(selp.SrcPubkey()),
		GasLimit:    int64(selp.GasLimit()),
		Signature:   hex.EncodeToString(selp.Signature()),
	}
	if selp.GasPrice() != nil {
		request.GasPrice = selp.GasPrice().String()
	}
	bo := backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Duration(retryInterval)*time.Second), uint64(retryNum))
	if err := backoff.Retry(func() error {
		_, err := c.SendVote(request)
		return err
	}, bo); err != nil {
		log.L().Error("Failed to inject vote", zap.Error(err))
	}
	log.S().Infof("Sent out the signed vote: %+v", request)

	if wg != nil {
		wg.Done()
	}
}

func injectExecInteraction(
	wg *sync.WaitGroup,
	c explorer.Explorer,
	executor *AddressKey,
	contract string,
	nonce uint64,
	amount *big.Int,
	gasLimit uint64,
	gasPrice *big.Int,
	data string,
	retryNum int,
	retryInterval int,
) {
	selp, execution, err := createSignedExecution(executor, contract, nonce, amount, gasLimit, gasPrice, data)
	if err != nil {
		log.L().Fatal("Failed to inject execution", zap.Error(err))
	}

	log.L().Info("Created signed execution")

	injectExecution(selp, execution, c, retryNum, retryInterval)
	if wg != nil {
		wg.Done()
	}
}

// Helper function to get the sender, recipient, and nonce of next injected transfer
func createTransferInjection(
	counter map[string]uint64,
	addrs []*AddressKey,
) (*AddressKey, *AddressKey, uint64) {
	sender := addrs[rand.Intn(len(addrs))]
	recipient := addrs[rand.Intn(len(addrs))]
	nonce := counter[sender.EncodedAddr]
	counter[sender.EncodedAddr]++
	return sender, recipient, nonce
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
		SetDestinationAddress(recipient.EncodedAddr).
		SetGasLimit(gasLimit).
		SetAction(transfer).Build()
	selp, err := action.Sign(elp, sender.EncodedAddr, sender.PriKey)
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
		SetDestinationAddress(votee.EncodedAddr).
		SetGasLimit(gasLimit).
		SetAction(vote).Build()
	selp, err := action.Sign(elp, voter.EncodedAddr, voter.PriKey)
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
		SetDestinationAddress(contract).
		SetGasLimit(gasLimit).
		SetAction(execution).Build()
	selp, err := action.Sign(elp, executor.EncodedAddr, executor.PriKey)
	if err != nil {
		return action.SealedEnvelope{}, nil, errors.Wrapf(err, "failed to sign execution %v", elp)
	}
	return selp, execution, nil
}

func injectExecution(
	selp action.SealedEnvelope,
	execution *action.Execution,
	c explorer.Explorer,
	retryNum int,
	retryInterval int,
) {
	request := explorer.Execution{
		Version:        int64(selp.Version()),
		Nonce:          int64(selp.Nonce()),
		Contract:       selp.DstAddr(),
		ExecutorPubKey: keypair.EncodePublicKey(selp.SrcPubkey()),
		GasLimit:       int64(selp.GasLimit()),
		Data:           hex.EncodeToString(execution.Data()),
		Signature:      hex.EncodeToString(selp.Signature()),
	}
	if execution.Amount() != nil {
		request.Amount = execution.Amount().String()
	}
	if selp.GasPrice() != nil {
		request.GasPrice = selp.GasPrice().String()
	}

	bo := backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Duration(retryInterval)*time.Second), uint64(retryNum))
	if err := backoff.Retry(func() error {
		_, err := c.SendSmartContract(request)
		return err
	}, bo); err != nil {
		log.L().Error("Failed to inject execution", zap.Error(err))
	}
	log.S().Infof("Sent out the signed execution: %+v", request)
}
