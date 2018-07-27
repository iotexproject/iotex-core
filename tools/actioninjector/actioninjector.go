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
	"encoding/json"
	"flag"
	"io/ioutil"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/explorer"
	exp "github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/testutil"
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
	// target address for jrpc connection. Default is "127.0.0.1:14004"
	var addr string
	// number of transfer injections. Default is 50
	var transferNum int
	// number of vote injections. Default is 50
	var voteNum int
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

	flag.StringVar(&configPath, "injector-config-path", "./tools/actioninjector/gentsfaddrs.yaml", "path of config file of genesis transfer addresses")
	flag.StringVar(&addr, "addr", "127.0.0.1:14004", "target ip:port for jrpc connection")
	flag.IntVar(&transferNum, "transfer-num", 50, "number of transfer injections")
	flag.IntVar(&voteNum, "vote-num", 50, "number of vote injections")
	flag.IntVar(&interval, "interval", 5, "sleep interval between two consecutively injected actions in seconds")
	flag.IntVar(&retryNum, "retry-num", 5, "maximum number of rpc retries")
	flag.IntVar(&retryInterval, "retry-interval", 1, "sleep interval between two consecutive rpc retries in seconds")
	flag.IntVar(&aps, "aps", 0, "actions to be injected per second")
	flag.IntVar(&duration, "duration", 60, "duration when the injection will run in seconds")
	flag.Parse()

	proxy := explorer.NewExplorerProxy("http://" + addr)

	// Load Senders' public/private key pairs
	addrBytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to start injecting actions")
	}
	addresses := Addresses{}
	err = yaml.Unmarshal(addrBytes, &addresses)
	if err := yaml.Unmarshal(addrBytes, &addresses); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start injecting actions")
	}

	// Construct iotex addresses for loaded senders
	addrs := []*iotxaddress.Address{}
	for _, pkPair := range addresses.PKPairs {
		addr := testutil.ConstructAddress(pkPair.PubKey, pkPair.PriKey)
		addrs = append(addrs, addr)
	}

	// Initiate the map of nonce counter
	counter := make(map[string]uint64)
	candidates := make(map[string]bool)
	for _, addr := range addrs {
		addrDetails, err := proxy.GetAddressDetails(addr.RawAddress)
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to start injecting actions")
		}
		nonce := uint64(addrDetails.PendingNonce)
		counter[addr.RawAddress] = nonce
		if addrDetails.IsCandidate {
			candidates[addr.RawAddress] = true
		}
	}

	rand.Seed(time.Now().UnixNano())

	// APS Mode
	if aps > 0 {
		d := time.Duration(duration) * time.Second
		wg := &sync.WaitGroup{}
		injectByAps(wg, aps, counter, proxy, addrs, d, candidates, retryNum, retryInterval)
		wg.Wait()
	} else {
		injectByInterval(transferNum, voteNum, interval, counter, proxy, addrs, candidates, retryNum, retryInterval)
	}
}

// Inject Actions in APS Mode
func injectByAps(
	wg *sync.WaitGroup,
	aps int,
	counter map[string]uint64,
	client exp.Explorer,
	addrs []*iotxaddress.Address,
	duration time.Duration,
	candidates map[string]bool,
	retryNum int,
	retryInterval int,
) {
	timeout := time.After(duration)
	tick := time.Tick(time.Duration(1/float64(aps)*1000) * time.Millisecond)
loop:
	for {
		select {
		case <-timeout:
			break loop
		case <-tick:
			// TODO: Revive vote injection
			wg.Add(1)
			rand := rand.Intn(2)
			if true || rand%2 == 1 {
				sender, recipient, nonce := createTransferInjection(counter, addrs)
				go injectTransfer(wg, client, sender, recipient, nonce, retryNum, retryInterval)
			} else {
				//sender, recipient, nonce := createVoteInjection(counter, addrs, candidates)
				//go injectVote(wg, client, sender, recipient, nonce, retryNum, retryInterval)
			}
		}
	}
}

// Inject Actions in Interval Mode
func injectByInterval(
	transferNum int,
	voteNum int,
	interval int,
	counter map[string]uint64,
	client exp.Explorer,
	addrs []*iotxaddress.Address,
	candidates map[string]bool,
	retryNum int,
	retryInterval int,
) {
	for transferNum > 0 && voteNum > 0 {
		sender, recipient, nonce := createTransferInjection(counter, addrs)
		injectTransfer(nil, client, sender, recipient, nonce, retryNum, retryInterval)
		time.Sleep(time.Second * time.Duration(interval))

		sender, recipient, nonce = createVoteInjection(counter, addrs, candidates)
		injectVote(nil, client, sender, recipient, nonce, retryNum, retryInterval)
		time.Sleep(time.Second * time.Duration(interval))
		transferNum--
		voteNum--
	}
	switch {
	case transferNum > 0:
		for transferNum > 0 {
			sender, recipient, nonce := createTransferInjection(counter, addrs)
			injectTransfer(nil, client, sender, recipient, nonce, retryNum, retryInterval)
			time.Sleep(time.Second * time.Duration(interval))
			transferNum--
		}
	case voteNum > 0:
		for voteNum > 0 {
			sender, recipient, nonce := createVoteInjection(counter, addrs, candidates)
			injectVote(nil, client, sender, recipient, nonce, retryNum, retryInterval)
			time.Sleep(time.Second * time.Duration(interval))
			voteNum--
		}
	}
}

func injectTransfer(
	wg *sync.WaitGroup,
	c exp.Explorer,
	sender *iotxaddress.Address,
	recipient *iotxaddress.Address,
	nonce uint64,
	retryNum int,
	retryInterval int,
) {
	amount := int64(0)
	for amount == int64(0) {
		amount = int64(rand.Intn(5))
	}

	var r exp.CreateRawTransferResponse
	var err error
	for i := 0; i < retryNum; i++ {
		if r, err = c.CreateRawTransfer(exp.CreateRawTransferRequest{Sender: sender.RawAddress, Recipient: recipient.RawAddress, Amount: amount, Nonce: int64(nonce)}); err == nil {
			break
		}
		time.Sleep(time.Duration(retryInterval) * time.Second)
	}
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject transfer")
	}
	logger.Info().Msg("Created raw transfer")

	tsf := &exp.Transfer{}
	serializedTransfer, err := hex.DecodeString(r.SerializedTransfer)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject transfer")
	}
	if err := json.Unmarshal(serializedTransfer, tsf); err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject transfer")
	}

	// Sign Transfer
	transfer := action.NewTransfer(uint64(tsf.Nonce), big.NewInt(tsf.Amount), tsf.Sender, tsf.Recipient)
	transfer, err = transfer.Sign(sender)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject transfer")
	}
	tsf.SenderPubKey = keypair.EncodePublicKey(transfer.SenderPublicKey)
	tsf.Signature = hex.EncodeToString(transfer.Signature)

	stsf, err := json.Marshal(tsf)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject transfer")
	}

	for i := 0; i < retryNum; i++ {
		if _, err = c.SendTransfer(exp.SendTransferRequest{hex.EncodeToString(stsf[:])}); err == nil {
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
	logger.Info().Int64("Amount", tsf.Amount).Msg(" ")
	logger.Info().Str("Sender", tsf.Sender).Msg(" ")
	logger.Info().Str("Recipient", tsf.Recipient).Msg(" ")
	logger.Info().Str("Payload", tsf.Payload).Msg(" ")
	logger.Info().Str("Sender Public Key", tsf.SenderPubKey).Msg(" ")
	logger.Info().Str("Signature", tsf.Signature).Msg(" ")
	logger.Info().Bool("IsCoinbase", tsf.IsCoinbase).Msg(" ")

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
	retryNum int,
	retryInterval int,
) {
	var r exp.CreateRawVoteResponse
	var err error
	for i := 0; i < retryNum; i++ {
		if r, err = c.CreateRawVote(exp.CreateRawVoteRequest{Voter: keypair.EncodePublicKey(sender.PublicKey), Votee: keypair.EncodePublicKey(recipient.PublicKey), Nonce: int64(nonce)}); err == nil {
			break
		}
		time.Sleep(time.Duration(retryInterval) * time.Second)
	}
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject vote")
	}
	logger.Info().Msg("Created raw vote")

	jsonVote := &exp.Vote{}
	serializedVote, err := hex.DecodeString(r.SerializedVote)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject vote")
	}
	if err := json.Unmarshal(serializedVote, jsonVote); err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject vote")
	}

	// Sign Vote
	voterPubKey, err := keypair.DecodePublicKey(jsonVote.VoterPubKey)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject vote")
	}
	voteePubKey, err := keypair.DecodePublicKey(jsonVote.VoteePubKey)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject vote")
	}
	vote := action.NewVote(uint64(jsonVote.Nonce), voterPubKey, voteePubKey)
	vote, err = vote.Sign(sender)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject vote")
	}
	jsonVote.Signature = hex.EncodeToString(vote.Signature)

	svote, err := json.Marshal(jsonVote)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject vote")
	}

	for i := 0; i < retryNum; i++ {
		if _, err = c.SendVote(exp.SendVoteRequest{SerializedVote: hex.EncodeToString(svote[:])}); err == nil {
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
	logger.Info().Str("Recipient Public Key", jsonVote.VoteePubKey).Msg(" ")
	logger.Info().Str("Signature", jsonVote.Signature).Msg(" ")

	if wg != nil {
		wg.Done()
	}
}

// Helper function to get the nonce of next injected transfer
func createTransferInjection(counter map[string]uint64, addrs []*iotxaddress.Address) (*iotxaddress.Address, *iotxaddress.Address, uint64) {
	sender := addrs[rand.Intn(len(addrs))]
	recipient := addrs[rand.Intn(len(addrs))]
	nonce := counter[sender.RawAddress]
	counter[sender.RawAddress]++
	return sender, recipient, nonce
}

// Helper function to get the nonce of next injected vote
func createVoteInjection(counter map[string]uint64, addrs []*iotxaddress.Address, candidates map[string]bool) (*iotxaddress.Address, *iotxaddress.Address, uint64) {
	sender := addrs[rand.Intn(len(addrs))]
	nextCandidates := []*iotxaddress.Address{}
	for _, address := range addrs {
		if address.RawAddress == sender.RawAddress {
			nextCandidates = append(nextCandidates, address)
			continue
		}
		if _, ok := candidates[address.RawAddress]; ok {
			nextCandidates = append(nextCandidates, address)
		}
	}
	recipient := nextCandidates[rand.Intn(len(nextCandidates))]
	candidates[recipient.RawAddress] = true
	nonce := counter[sender.RawAddress]
	counter[sender.RawAddress]++
	return sender, recipient, nonce
}
