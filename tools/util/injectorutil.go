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
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
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
	aps int,
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
	tick := time.Tick(time.Duration(1/float64(aps)*1000) * time.Millisecond)
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
				go injectExecution(wg, client, executor, contract, nonce, big.NewInt(int64(executionAmount)),
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
		injectExecution(nil, client, executor, contract, nonce, big.NewInt(int64(executionAmount)),
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
			injectExecution(nil, client, executor, contract, nonce, big.NewInt(int64(executionAmount)),
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
			injectExecution(nil, client, executor, contract, nonce, big.NewInt(int64(executionAmount)),
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
			injectExecution(nil, client, executor, contract, nonce, big.NewInt(int64(executionAmount)),
				uint64(executionGasLimit), big.NewInt(int64(executionGasPrice)), executionData, retryNum, retryInterval)
			time.Sleep(time.Second * time.Duration(interval))
			executionNum--
		}
	}
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

	transfer, err := createSignedTransfer(sender, recipient, big.NewInt(amount), nonce, gasLimit, gasPrice, payload)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject transfer")
	}

	logger.Info().Msg("Created signed transfer")

	tsf := transfer.ToJSON()
	request := explorer.SendTransferRequest{
		Version:      tsf.Version,
		Nonce:        tsf.Nonce,
		Sender:       tsf.Sender,
		Recipient:    tsf.Recipient,
		Amount:       tsf.Amount,
		SenderPubKey: tsf.SenderPubKey,
		GasLimit:     tsf.GasLimit,
		GasPrice:     tsf.GasPrice,
		Signature:    tsf.Signature,
		Payload:      tsf.Payload,
	}
	for i := 0; i < retryNum; i++ {
		if _, err = c.SendTransfer(request); err == nil {
			break
		}
		time.Sleep(time.Duration(retryInterval) * time.Second)
	}
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject transfer")
	}
	logger.Info().Msg("Sent out the signed transfer: ")

	logger.Info().Int64("Version", tsf.Version).Msg(" ")
	logger.Info().Int64("Nonce", tsf.Nonce).Msg(" ")
	logger.Info().Int64("amount", tsf.Amount).Msg(" ")
	logger.Info().Str("Sender", tsf.Sender).Msg(" ")
	logger.Info().Str("Recipient", tsf.Recipient).Msg(" ")
	logger.Info().Str("payload", tsf.Payload).Msg(" ")
	logger.Info().Str("Sender Public Key", tsf.SenderPubKey).Msg(" ")
	logger.Info().Int64("Gas Limit", tsf.GasLimit).Msg(" ")
	logger.Info().Int64("Gas Price", tsf.GasPrice).Msg(" ")
	logger.Info().Str("Signature", tsf.Signature).Msg(" ")
	logger.Info().Bool("isCoinbase", tsf.IsCoinbase).Msg(" ")

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
	vote, err := createSignedVote(sender, recipient, nonce, gasLimit, gasPrice)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject vote")
	}

	logger.Info().Msg("Created signed vote")

	jsonVote, err := vote.ToJSON()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject vote")
	}
	request := explorer.SendVoteRequest{
		Version:     jsonVote.Version,
		Nonce:       jsonVote.Nonce,
		Voter:       jsonVote.Voter,
		Votee:       jsonVote.Votee,
		VoterPubKey: jsonVote.VoterPubKey,
		GasLimit:    jsonVote.GasLimit,
		GasPrice:    jsonVote.GasPrice,
		Signature:   jsonVote.Signature,
	}
	for i := 0; i < retryNum; i++ {
		if _, err = c.SendVote(request); err == nil {
			break
		}
		time.Sleep(time.Duration(retryInterval) * time.Second)
	}
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject vote")
	}
	logger.Info().Msg("Sent out the signed vote: ")

	logger.Info().Int64("Version", jsonVote.Version).Msg(" ")
	logger.Info().Int64("Nonce", jsonVote.Nonce).Msg(" ")
	logger.Info().Str("Sender Public Key", jsonVote.VoterPubKey).Msg(" ")
	logger.Info().Str("Recipient Address", jsonVote.Votee).Msg(" ")
	logger.Info().Int64("Gas Limit", jsonVote.GasLimit)
	logger.Info().Int64("Gas Price", jsonVote.GasLimit)
	logger.Info().Str("Signature", jsonVote.Signature).Msg(" ")

	if wg != nil {
		wg.Done()
	}
}

func injectExecution(
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
	execution, err := createSignedExecution(executor, contract, nonce, amount, gasLimit, gasPrice, data)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject execution")
	}

	logger.Info().Msg("Created signed execution")

	jsonExecution, err := execution.ToJSON()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject execution")
	}
	for i := 0; i < retryNum; i++ {
		if _, err = c.SendSmartContract(*jsonExecution); err == nil {
			break
		}
		time.Sleep(time.Duration(retryInterval) * time.Second)
	}
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject execution")
	}
	logger.Info().Msg("Sent out the signed execution: ")

	logger.Info().Int64("Version", jsonExecution.Version).Msg(" ")
	logger.Info().Int64("Nonce", jsonExecution.Nonce).Msg(" ")
	logger.Info().Int64("amount", jsonExecution.Amount).Msg(" ")
	logger.Info().Str("Executor", jsonExecution.Executor).Msg(" ")
	logger.Info().Str("Contract", jsonExecution.Contract).Msg(" ")
	logger.Info().Int64("Gas", jsonExecution.GasLimit).Msg(" ")
	logger.Info().Int64("Gas Price", jsonExecution.GasPrice).Msg(" ")
	logger.Info().Str("data", jsonExecution.Data)
	logger.Info().Str("Signature", jsonExecution.Signature).Msg(" ")

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
) (*action.Transfer, error) {
	transferPayload, err := hex.DecodeString(payload)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode payload %s", payload)
	}
	transfer, err := action.NewTransfer(
		nonce, amount, sender.RawAddress, recipient.RawAddress, transferPayload, gasLimit, gasPrice)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create raw transfer")
	}
	if err := action.Sign(transfer, sender.PrivateKey); err != nil {
		return nil, errors.Wrapf(err, "failed to sign transfer %v", transfer)
	}
	return transfer, nil
}

// Helper function to create and sign a vote
func createSignedVote(
	voter *iotxaddress.Address,
	votee *iotxaddress.Address,
	nonce uint64,
	gasLimit uint64,
	gasPrice *big.Int,
) (*action.Vote, error) {
	vote, err := action.NewVote(nonce, voter.RawAddress, votee.RawAddress, gasLimit, gasPrice)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create raw vote")
	}
	if err := action.Sign(vote, voter.PrivateKey); err != nil {
		return nil, errors.Wrapf(err, "failed to sign vote %v", vote)
	}
	return vote, nil
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
) (*action.Execution, error) {
	executionData, err := hex.DecodeString(data)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode data %s", data)
	}
	execution, err := action.NewExecution(executor.RawAddress, contract, nonce, amount,
		gasLimit, gasPrice, executionData)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create raw execution")
	}
	if err := action.Sign(execution, executor.PrivateKey); err != nil {
		return nil, errors.Wrapf(err, "failed to sign execution %v", execution)
	}
	return execution, nil
}
