// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// This is a testing tool to inject fake actions to the blockchain
// To use, run "make build" and " ./bin/actioninjector"

package main

import (
	"encoding/hex"
	"flag"
	"io/ioutil"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/pkg/errors"
	_ "go.uber.org/automaxprocs"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/explorer"
	exp "github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	adminNumber = 2
)

// Addresses indicate the addresses getting transfers from Creator in genesis block
type Addresses struct {
	PKPairs []PKPair `yaml:"pkPairs"`
}

// PKPair contains the public and private key of an address
type PKPair struct {
	PubKey string `yaml:"pubKey"`
	PriKey string `yaml:"priKey"`
}

func main() {
	// path of config file containing all the public/private key paris of addresses getting transfers from Creator in genesis block
	var configPath string
	// chain ID. Default is 1
	var chainID int
	// target address for jrpc connection. Default is "127.0.0.1:14004"
	var addr string
	// number of transfer injections. Default is 50
	var transferNum int
	// transfer gas limit. Default is 1000000
	var transferGasLimit int
	// transfer gas price. Default is 10
	var transferGasPrice int
	// transfer payload. Default is ""
	var transferPayload string
	// number of vote injections. Default is 50
	var voteNum int
	// vote gas limit. Default is 1000000
	var voteGasLimit int
	// vote gas price. Default is 10
	var voteGasPrice int
	// number of execution injections. Default is 50
	var executionNum int
	// smart contract address. Default is "io1qyqsyqcy3kcd2pyfwus69nzgvkwhg8mk8h336dt86pg6cj"
	var contract string
	// execution amount. Default is 0
	var executionAmount int
	// execution gas limit. Default is 1200000
	var executionGasLimit int
	// execution gas price. Default is 10
	var executionGasPrice int
	// execution data. Default is "2885ad2c"
	var executionData string
	// sleeping period between every two consecutive action injections in seconds. Default is 5
	var interval int
	// maximum number of rpc retries. Default is 5
	var retryNum int
	// sleeping period between two consecutive rpc retries in seconds. Default is 1
	var retryInterval int
	// aps indicates how many actions to be injected in one second. Default is 0
	var aps int
	// duration indicates how long the injection will run in seconds. Default is 60
	var duration int
	// reset interval indicates the interval to reset nonce counter in seconds. Default is 10
	var resetInterval int

	flag.StringVar(&configPath, "injector-config-path", "./tools/actioninjector/gentsfaddrs.yaml", "path of config file of genesis transfer addresses")
	flag.IntVar(&chainID, "chain", 1, "id of target chain")
	flag.StringVar(&addr, "addr", "127.0.0.1:14004", "target ip:port for jrpc connection")
	flag.IntVar(&transferNum, "transfer-num", 50, "number of transfer injections")
	flag.IntVar(&transferGasLimit, "transfer-gas-limit", 1000000, "transfer gas limit")
	flag.IntVar(&transferGasPrice, "transfer-gas-price", 10, "transfer gas price")
	flag.StringVar(&transferPayload, "transfer-payload", "", "transfer payload")
	flag.IntVar(&voteNum, "vote-num", 50, "number of vote injections")
	flag.IntVar(&voteGasLimit, "vote-gas-limit", 1000000, "vote gas limit")
	flag.IntVar(&voteGasPrice, "vote-gas-price", 10, "vote gas price")
	flag.IntVar(&executionNum, "execution-num", 50, "number of execution injections")
	flag.StringVar(&contract, "contract", "io1qyqsyqcy3kcd2pyfwus69nzgvkwhg8mk8h336dt86pg6cj", "smart contract address")
	flag.IntVar(&executionAmount, "execution-amount", 50, "execution amount")
	flag.IntVar(&executionGasLimit, "execution-gas-limit", 1200000, "execution gas limit")
	flag.IntVar(&executionGasPrice, "execution-gas-price", 10, "execution gas price")
	flag.StringVar(&executionData, "execution-data", "2885ad2c", "execution data")
	flag.IntVar(&interval, "interval", 5, "sleep interval between two consecutively injected actions in seconds")
	flag.IntVar(&retryNum, "retry-num", 5, "maximum number of rpc retries")
	flag.IntVar(&retryInterval, "retry-interval", 1, "sleep interval between two consecutive rpc retries in seconds")
	flag.IntVar(&aps, "aps", 0, "actions to be injected per second")
	flag.IntVar(&duration, "duration", 60, "duration when the injection will run in seconds")
	flag.IntVar(&resetInterval, "reset-interval", 10, "time interval to reset nonce counter in seconds")
	flag.Parse()

	proxy := explorer.NewExplorerProxy("http://" + addr)

	// Load Senders' public/private key pairs
	addrBytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to start injecting actions")
	}
	var addresses Addresses
	if err := yaml.Unmarshal(addrBytes, &addresses); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start injecting actions")
	}

	// Construct iotex addresses for loaded senders
	addrs := []*iotxaddress.Address{}
	for _, pkPair := range addresses.PKPairs {
		addr := testutil.ConstructAddress(uint32(chainID), pkPair.PubKey, pkPair.PriKey)
		addrs = append(addrs, addr)
	}
	admins := addrs[len(addrs)-adminNumber:]
	delegates := addrs[:len(addrs)-adminNumber]

	// Initiate the map of nonce counter
	counter := make(map[string]uint64)
	for _, addr := range addrs {
		addrDetails, err := proxy.GetAddressDetails(addr.RawAddress)
		if err != nil {
			logger.Fatal().Err(err).Str("addr", addr.RawAddress).Msg("Failed to start injecting actions")
		}
		nonce := uint64(addrDetails.PendingNonce)
		counter[addr.RawAddress] = nonce
	}

	rand.Seed(time.Now().UnixNano())

	// APS Mode
	if aps > 0 {
		d := time.Duration(duration) * time.Second
		wg := &sync.WaitGroup{}
		injectByAps(wg, aps, counter, transferGasLimit, transferGasPrice, transferPayload, voteGasLimit, voteGasPrice,
			contract, executionAmount, executionGasLimit, executionGasPrice, executionData, proxy, admins, delegates, d,
			retryNum, retryInterval, resetInterval)
		wg.Wait()
	} else {
		injectByInterval(transferNum, transferGasLimit, transferGasPrice, transferPayload, voteNum, voteGasLimit,
			voteGasPrice, executionNum, contract, executionAmount, executionGasLimit, executionGasPrice, executionData,
			interval, counter, proxy, admins, delegates, retryNum, retryInterval)
	}
}

// Inject Actions in APS Mode
func injectByAps(
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
	client exp.Explorer,
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
loop:
	for {
		select {
		case <-timeout:
			break loop
		case <-reset:
			for _, admin := range admins {
				addrDetails, err := client.GetAddressDetails(admin.RawAddress)
				if err != nil {
					logger.Fatal().Err(err).Str("addr", admin.RawAddress).Msg("Failed to inject actions by APS")
				}
				nonce := uint64(addrDetails.PendingNonce)
				counter[admin.RawAddress] = nonce
			}
			for _, delegate := range delegates {
				addrDetails, err := client.GetAddressDetails(delegate.RawAddress)
				if err != nil {
					logger.Fatal().Err(err).Str("addr", delegate.RawAddress).Msg("Failed to inject actions by APS")
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
					uint64(executionGasLimit), big.NewInt(int64(executionGasPrice)), executionData, retryNum, retryInterval)
			}
		}
	}
}

// Inject Actions in Interval Mode
func injectByInterval(
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
	client exp.Explorer,
	admins []*iotxaddress.Address,
	delegates []*iotxaddress.Address,
	retryNum int,
	retryInterval int,
) {
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
	c exp.Explorer,
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
	request := exp.SendTransferRequest{
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
	c exp.Explorer,
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
	request := exp.SendVoteRequest{
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
	c exp.Explorer,
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
func createTransferInjection(counter map[string]uint64, addrs []*iotxaddress.Address) (*iotxaddress.Address, *iotxaddress.Address, uint64) {
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
func createExecutionInjection(counter map[string]uint64, addrs []*iotxaddress.Address) (*iotxaddress.Address, uint64) {
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
	execution, err := action.NewExecution(executor.RawAddress, contract, nonce, amount, gasLimit, gasPrice, executionData)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create raw execution")
	}
	if err := action.Sign(execution, executor.PrivateKey); err != nil {
		return nil, errors.Wrapf(err, "failed to sign execution %v", execution)
	}
	return execution, nil
}
