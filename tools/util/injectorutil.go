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

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/test/testaddress"
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

// LoadAddresses loads key pairs from key pair path and construct addresses
func LoadAddresses(keypairsPath string, chainID uint32) ([]*iotxaddress.Address, error) {
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
	addrs := make([]*iotxaddress.Address, 0)
	for _, pair := range keypairs.Pairs {
		addr := testaddress.ConstructAddress(chainID, pair.PK, pair.SK)
		addrs = append(addrs, addr)
	}
	return addrs, nil
}

// InitCounter initializes the map of nonce counter of each address
func InitCounter(client explorer.Explorer, addrs []*iotxaddress.Address) (map[string]uint64, error) {
	counter := make(map[string]uint64)
	for _, addr := range addrs {
		addrDetails, err := client.GetAddressDetails(addr.RawAddress)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get address details of %s", addr.RawAddress)
		}
		nonce := uint64(addrDetails.PendingNonce)
		counter[addr.RawAddress] = nonce
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
	admins []*iotxaddress.Address,
	delegates []*iotxaddress.Address,
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
				addrDetails, err := client.GetAddressDetails(admin.RawAddress)
				if err != nil {
					logger.Fatal().Err(err).Str("addr", admin.RawAddress).
						Msg("Failed to inject actions by APS")
				}
				nonce := uint64(addrDetails.PendingNonce)
				counter[admin.RawAddress] = nonce
			}
			for _, delegate := range delegates {
				addrDetails, err := client.GetAddressDetails(delegate.RawAddress)
				if err != nil {
					logger.Fatal().Err(err).Str("addr", delegate.RawAddress).
						Msg("Failed to inject actions by APS")
				}
				nonce := uint64(addrDetails.PendingNonce)
				counter[delegate.RawAddress] = nonce
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
	admins []*iotxaddress.Address,
	delegates []*iotxaddress.Address,
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
	delegates []*iotxaddress.Address,
	executionGasLimit int,
	executionGasPrice int,
	executionData string,
	retryNum int,
	retryInterval int,
) (hash.Hash32B, error) {
	executor, nonce := createExecutionInjection(counter, delegates)
	selp, execution, err := createSignedExecution(executor, action.EmptyAddress, nonce, big.NewInt(0),
		uint64(executionGasLimit), big.NewInt(int64(executionGasPrice)), executionData)
	if err != nil {
		return hash.ZeroHash32B, errors.Wrap(err, "failed to create signed execution")
	}
	logger.Info().Msg("Created signed execution")

	injectExecution(selp, execution, client, retryNum, retryInterval)
	return selp.Hash(), nil
}

func injectTransfer(
	wg *sync.WaitGroup,
	c explorer.Explorer,
	sender *iotxaddress.Address,
	recipient *iotxaddress.Address,
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
		logger.Fatal().Err(err).Msg("Failed to inject transfer")
	}

	logger.Info().Msg("Created signed transfer")

	request := explorer.SendTransferRequest{
		Version:      int64(selp.Version()),
		Nonce:        int64(selp.Nonce()),
		Sender:       selp.SrcAddr(),
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
	for i := 0; i < retryNum; i++ {
		if _, err = c.SendTransfer(request); err == nil {
			break
		}
		time.Sleep(time.Duration(retryInterval) * time.Second)
	}
	if err != nil {
		logger.Error().Err(err).Msg("Failed to inject transfer")
	}
	logger.Info().Msg("Sent out the signed transfer: ")

	logger.Info().Int64("Version", request.Version).Msg(" ")
	logger.Info().Int64("Nonce", request.Nonce).Msg(" ")
	logger.Info().Str("amount", request.Amount).Msg(" ")
	logger.Info().Str("Sender", request.Sender).Msg(" ")
	logger.Info().Str("Recipient", request.Recipient).Msg(" ")
	logger.Info().Str("payload", request.Payload).Msg(" ")
	logger.Info().Str("Sender Public Key", request.SenderPubKey).Msg(" ")
	logger.Info().Int64("Gas Limit", request.GasLimit).Msg(" ")
	logger.Info().Str("Gas Price", request.GasPrice).Msg(" ")
	logger.Info().Str("Signature", request.Signature).Msg(" ")
	logger.Info().Bool("isCoinbase", request.IsCoinbase).Msg(" ")

	if wg != nil {
		wg.Done()
	}
}

func injectVote(
	wg *sync.WaitGroup,
	c explorer.Explorer,
	sender *iotxaddress.Address,
	recipient *iotxaddress.Address,
	nonce uint64,
	gasLimit uint64,
	gasPrice *big.Int,
	retryNum int,
	retryInterval int,
) {
	selp, _, err := createSignedVote(sender, recipient, nonce, gasLimit, gasPrice)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject vote")
	}

	logger.Info().Msg("Created signed vote")

	request := explorer.SendVoteRequest{
		Version:     int64(selp.Version()),
		Nonce:       int64(selp.Nonce()),
		Voter:       selp.SrcAddr(),
		Votee:       selp.DstAddr(),
		VoterPubKey: keypair.EncodePublicKey(selp.SrcPubkey()),
		GasLimit:    int64(selp.GasLimit()),
		Signature:   hex.EncodeToString(selp.Signature()),
	}
	if selp.GasPrice() != nil {
		request.GasPrice = selp.GasPrice().String()
	}
	for i := 0; i < retryNum; i++ {
		if _, err = c.SendVote(request); err == nil {
			break
		}
		time.Sleep(time.Duration(retryInterval) * time.Second)
	}
	if err != nil {
		logger.Error().Err(err).Msg("Failed to inject vote")
	}
	logger.Info().Msg("Sent out the signed vote: ")

	logger.Info().Int64("Version", request.Version).Msg(" ")
	logger.Info().Int64("Nonce", request.Nonce).Msg(" ")
	logger.Info().Str("Sender Public Key", request.VoterPubKey).Msg(" ")
	logger.Info().Str("Recipient Address", request.Votee).Msg(" ")
	logger.Info().Int64("Gas Limit", request.GasLimit)
	logger.Info().Str("Gas Price", request.GasPrice)
	logger.Info().Str("Signature", request.Signature).Msg(" ")

	if wg != nil {
		wg.Done()
	}
}

func injectExecInteraction(
	wg *sync.WaitGroup,
	c explorer.Explorer,
	executor *iotxaddress.Address,
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
		logger.Fatal().Err(err).Msg("Failed to inject execution")
	}

	logger.Info().Msg("Created signed execution")

	injectExecution(selp, execution, c, retryNum, retryInterval)
	if wg != nil {
		wg.Done()
	}
}

// Helper function to get the sender, recipient, and nonce of next injected transfer
func createTransferInjection(
	counter map[string]uint64,
	addrs []*iotxaddress.Address,
) (*iotxaddress.Address, *iotxaddress.Address, uint64) {
	sender := addrs[rand.Intn(len(addrs))]
	recipient := addrs[rand.Intn(len(addrs))]
	nonce := counter[sender.RawAddress]
	counter[sender.RawAddress]++
	return sender, recipient, nonce
}

// Helper function to get the sender, recipient, and nonce of next injected vote
func createVoteInjection(
	counter map[string]uint64,
	admins []*iotxaddress.Address,
	delegates []*iotxaddress.Address,
) (*iotxaddress.Address, *iotxaddress.Address, uint64) {
	sender := admins[rand.Intn(len(admins))]
	recipient := delegates[rand.Intn(len(delegates))]
	nonce := counter[sender.RawAddress]
	counter[sender.RawAddress]++
	return sender, recipient, nonce
}

// Helper function to get the executor and nonce of next injected execution
func createExecutionInjection(
	counter map[string]uint64,
	addrs []*iotxaddress.Address,
) (*iotxaddress.Address, uint64) {
	executor := addrs[rand.Intn(len(addrs))]
	nonce := counter[executor.RawAddress]
	counter[executor.RawAddress]++
	return executor, nonce
}

// Helper function to create and sign a transfer
func createSignedTransfer(
	sender *iotxaddress.Address,
	recipient *iotxaddress.Address,
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
		nonce, amount, sender.RawAddress, recipient.RawAddress, transferPayload, gasLimit, gasPrice)
	if err != nil {
		return action.SealedEnvelope{}, nil, errors.Wrap(err, "failed to create raw transfer")
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetDestinationAddress(recipient.RawAddress).
		SetGasLimit(gasLimit).
		SetAction(transfer).Build()
	selp, err := action.Sign(elp, sender.RawAddress, sender.PrivateKey)
	if err != nil {
		return action.SealedEnvelope{}, nil, errors.Wrapf(err, "failed to sign transfer %v", elp)
	}
	return selp, transfer, nil
}

// Helper function to create and sign a vote
func createSignedVote(
	voter *iotxaddress.Address,
	votee *iotxaddress.Address,
	nonce uint64,
	gasLimit uint64,
	gasPrice *big.Int,
) (action.SealedEnvelope, *action.Vote, error) {
	vote, err := action.NewVote(nonce, voter.RawAddress, votee.RawAddress, gasLimit, gasPrice)
	if err != nil {
		return action.SealedEnvelope{}, nil, errors.Wrap(err, "failed to create raw vote")
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetDestinationAddress(votee.RawAddress).
		SetGasLimit(gasLimit).
		SetAction(vote).Build()
	selp, err := action.Sign(elp, voter.RawAddress, voter.PrivateKey)
	if err != nil {
		return action.SealedEnvelope{}, nil, errors.Wrapf(err, "failed to sign vote %v", elp)
	}
	return selp, vote, nil
}

// Helper function to create and sign an execution
func createSignedExecution(
	executor *iotxaddress.Address,
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
	execution, err := action.NewExecution(executor.RawAddress, contract, nonce, amount,
		gasLimit, gasPrice, executionData)
	if err != nil {
		return action.SealedEnvelope{}, nil, errors.Wrap(err, "failed to create raw execution")
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetDestinationAddress(contract).
		SetGasLimit(gasLimit).
		SetAction(execution).Build()
	selp, err := action.Sign(elp, executor.RawAddress, executor.PrivateKey)
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
		Executor:       selp.SrcAddr(),
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
	var err error
	for i := 0; i < retryNum; i++ {
		if _, err = c.SendSmartContract(request); err == nil {
			break
		}
		time.Sleep(time.Duration(retryInterval) * time.Second)
	}
	if err != nil {
		logger.Error().Err(err).Msg("Failed to inject execution")
	}
	logger.Info().Msg("Sent out the signed execution: ")

	logger.Info().Int64("Version", request.Version).Msg(" ")
	logger.Info().Int64("Nonce", request.Nonce).Msg(" ")
	logger.Info().Str("amount", request.Amount).Msg(" ")
	logger.Info().Str("Executor", request.Executor).Msg(" ")
	logger.Info().Str("Contract", request.Contract).Msg(" ")
	logger.Info().Int64("Gas", request.GasLimit).Msg(" ")
	logger.Info().Str("Gas Price", request.GasPrice).Msg(" ")
	logger.Info().Str("data", request.Data)
	logger.Info().Str("Signature", request.Signature).Msg(" ")
}
