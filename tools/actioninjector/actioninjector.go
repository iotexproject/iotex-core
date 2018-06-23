// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// This is a testing tool to inject fake actions to the blockchain
// To use, run "make build" and " ./bin/actioninjector"

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/explorer"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	pb "github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/test/util"
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
	// target address for grpc connection. Default is "127.0.0.1:42124"
	var grpcAddr string
	// target address for jrpc connection. Default is "127.0.0.1:14004"
	var jrpcAddr string
	// number of transfer injections. Default is 50
	var transferNum int
	// number of vote injections. Default is 50
	var voteNum int
	// sleeping period between every two consecutive action injections in seconds. Default is 5
	var interval int
	// aps indicates how many actions to be injected in one second. Default is 0
	var aps int
	// duration indicates how long the injection will run in seconds. Default is 60
	var duration int
	flag.StringVar(&configPath, "config-path", "./tools/actioninjector/gentsfaddrs.yaml", "path of config file of genesis transfer addresses")
	flag.StringVar(&grpcAddr, "grpc-addr", "127.0.0.1:42124", "target ip:port for grpc connection")
	flag.StringVar(&jrpcAddr, "jrpc-addr", "127.0.0.1:14004", "target ip:port for jrpc connection")
	flag.IntVar(&transferNum, "transfer-num", 50, "number of transfer injections")
	flag.IntVar(&voteNum, "vote-num", 50, "number of vote injections")
	flag.IntVar(&interval, "interval", 5, "sleep interval of two consecutively injected actions in seconds")
	flag.IntVar(&aps, "aps", 0, "actions to be injected per second")
	flag.IntVar(&duration, "duration", 60, "duration when the injection will run in seconds")
	flag.Parse()
	conn, err := grpc.Dial(grpcAddr, grpc.WithInsecure())
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to start injecting actions")
	}
	defer conn.Close()

	client := pb.NewChainServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(interval*(transferNum+voteNum)))

	proxy := explorer.NewExplorerProxy("http://" + jrpcAddr)

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
		addr := util.ConstructAddress(pkPair.PubKey, pkPair.PriKey)
		addrs = append(addrs, addr)
	}

	// Initiate the map of nonce counter
	counter := make(map[string]uint64)
	for _, addr := range addrs {
		addrDetails, err := proxy.GetAddressDetails(addr.RawAddress)
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to start injecting actions")
		}
		nonce := uint64(addrDetails.Nonce + 1)
		counter[addr.RawAddress] = nonce
	}

	rand.Seed(time.Now().UnixNano())

	// APS Mode
	if aps > 0 {
		d := time.Duration(duration) * time.Second
		ctx, cancel = context.WithTimeout(context.Background(), d)
		defer cancel()
		wg := &sync.WaitGroup{}
		injectByAps(ctx, wg, aps, counter, client, addrs, d)
		wg.Wait()
	} else {
		if interval == 0 {
			ctx, cancel = context.WithTimeout(context.Background(), time.Second)
		}
		defer cancel()
		injectByInterval(ctx, transferNum, voteNum, interval, counter, client, addrs)
	}
}

// Inject Actions in APS Mode
func injectByAps(
	ctx context.Context,
	wg *sync.WaitGroup,
	aps int,
	counter map[string]uint64,
	client pb.ChainServiceClient,
	addrs []*iotxaddress.Address,
	duration time.Duration,
) {
	timeout := time.After(duration)
	tick := time.Tick(time.Duration(1/float64(aps)*1000) * time.Millisecond)
loop:
	for {
		select {
		case <-timeout:
			break loop
		case <-tick:
			wg.Add(1)
			sender, recipient, nonce := createInjection(counter, addrs)
			if nonce%2 == 1 {
				go injectTransfer(ctx, wg, client, sender, recipient, nonce)
			} else {
				go injectVote(ctx, wg, client, sender, recipient, nonce)
			}
		}
	}
}

// Inject Actions in Interval Mode
func injectByInterval(ctx context.Context, transferNum int, voteNum int, interval int, counter map[string]uint64, client pb.ChainServiceClient, addrs []*iotxaddress.Address) {
	for transferNum > 0 && voteNum > 0 {
		sender, recipient, nonce := createInjection(counter, addrs)
		injectTransfer(ctx, nil, client, sender, recipient, nonce)
		time.Sleep(time.Second * time.Duration(interval))

		sender, recipient, nonce = createInjection(counter, addrs)
		injectVote(ctx, nil, client, sender, recipient, nonce)
		time.Sleep(time.Second * time.Duration(interval))
		transferNum--
		voteNum--
	}
	switch {
	case transferNum > 0:
		for transferNum > 0 {
			sender, recipient, nonce := createInjection(counter, addrs)
			injectTransfer(ctx, nil, client, sender, recipient, nonce)
			time.Sleep(time.Second * time.Duration(interval))
			transferNum--
		}
	case voteNum > 0:
		for voteNum > 0 {
			sender, recipient, nonce := createInjection(counter, addrs)
			injectVote(ctx, nil, client, sender, recipient, nonce)
			time.Sleep(time.Second * time.Duration(interval))
			voteNum--
		}
	}
}

func injectTransfer(ctx context.Context, wg *sync.WaitGroup, c pb.ChainServiceClient, sender *iotxaddress.Address, recipient *iotxaddress.Address, nonce uint64) {
	amount := uint64(0)
	for amount == uint64(0) {
		amount = uint64(rand.Intn(5))
	}

	a := int64(amount)
	r, err := c.CreateRawTransfer(ctx, &pb.CreateRawTransferRequest{
		Sender: sender.RawAddress, Recipient: recipient.RawAddress, Amount: big.NewInt(a).Bytes(), Nonce: nonce, Data: []byte{}})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject transfer")
	}
	fmt.Println("Created raw transfer")

	tsf := &pb.TransferPb{}
	if err := proto.Unmarshal(r.SerializedTransfer, tsf); err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject transfer")
	}

	// Sign Transfer
	value := big.NewInt(0)
	transfer := action.NewTransfer(tsf.Nonce, value.SetBytes(tsf.Amount), tsf.Sender, tsf.Recipient)
	transfer, err = transfer.Sign(sender)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject transfer")
	}
	tsf.SenderPubKey = transfer.SenderPublicKey
	tsf.Signature = transfer.Signature

	stsf, err := proto.Marshal(tsf)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject transfer")
	}
	_, err = c.SendTransfer(ctx, &pb.SendTransferRequest{SerializedTransfer: stsf})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject transfer")
	}
	fmt.Println("Sent out the signed transfer: ")

	fmt.Printf("Version: %d\n", tsf.Version)
	fmt.Printf("Nonce: %d\n", tsf.Nonce)
	fmt.Printf("Amount: %x\n", tsf.Amount)
	fmt.Printf("Sender: %s\n", tsf.Sender)
	fmt.Printf("Recipient: %s\n", tsf.Recipient)
	fmt.Printf("Payload: %x\n", tsf.Payload)
	fmt.Printf("Sender Public Key: %x\n", tsf.SenderPubKey)
	fmt.Printf("Signature: %x\n", tsf.Signature)

	if wg != nil {
		wg.Done()
	}
}

func injectVote(ctx context.Context, wg *sync.WaitGroup, c pb.ChainServiceClient, sender *iotxaddress.Address, recipient *iotxaddress.Address, nonce uint64) {
	r, err := c.CreateRawVote(ctx, &pb.CreateRawVoteRequest{Voter: sender.PublicKey, Votee: recipient.PublicKey, Nonce: nonce})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject vote")
	}
	fmt.Println("Created raw vote")

	votePb := &pb.VotePb{}
	if err := proto.Unmarshal(r.SerializedVote, votePb); err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject vote")
	}

	// Sign Vote
	vote := action.NewVote(votePb.Nonce, votePb.SelfPubkey, votePb.VotePubkey)
	vote, err = vote.Sign(sender)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject vote")
	}
	votePb.Signature = vote.Signature

	svote, err := proto.Marshal(votePb)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject vote")
	}
	_, err = c.SendVote(ctx, &pb.SendVoteRequest{SerializedVote: svote})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to inject vote")
	}
	fmt.Println("Sent out the signed vote: ")

	fmt.Printf("Version: %d\n", votePb.Version)
	fmt.Printf("Nonce: %d\n", votePb.Nonce)
	fmt.Printf("Sender Public Key: %x\n", votePb.SelfPubkey)
	fmt.Printf("Recipient Public Key: %x\n", votePb.VotePubkey)
	fmt.Printf("Signature: %x\n", votePb.Signature)

	if wg != nil {
		wg.Done()
	}
}

// Helper function to get the nonce of next injected action
func createInjection(counter map[string]uint64, addrs []*iotxaddress.Address) (*iotxaddress.Address, *iotxaddress.Address, uint64) {
	sender := addrs[rand.Intn(10)]
	recipient := addrs[rand.Intn(10)]
	nonce := counter[sender.RawAddress]
	counter[sender.RawAddress]++
	return sender, recipient, nonce
}
