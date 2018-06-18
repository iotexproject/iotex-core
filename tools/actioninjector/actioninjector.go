// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// This is a testing tool to inject fake actions to the blockchain
// To use, run "make build" and " ./bin/actioninjector"

package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/explorer"
	"github.com/iotexproject/iotex-core/iotxaddress"
	pb "github.com/iotexproject/iotex-core/proto"
)

const (
	// Port number for grpc connection
	grpcPort = ":42124"
	// Port number for jrpc connection
	jrpcPort = ":14004"
	// Miner's public/private key pair is used as sender's key pair
	pubkeyMiner = "336eb60a5741f585a8e81de64e071327a3b96c15af4af5723598a07b6121e8e813bbd0056ba71ae29c0d64252e913f60afaeb11059908b81ff27cbfa327fd371d35f5ec0cbc01705"
	prikeyMiner = "925f0c9e4b6f6d92f2961d01aff6204c44d73c0b9d0da188582932d4fcad0d8ee8c66600"
	// Recipient of either a transfer or a vote would have the address constructed from one of the public/private key pairs below
	pubkeyA = "2c9ccbeb9ee91271f7e5c2103753be9c9edff847e1a51227df6a6b0765f31a4b424e84027b44a663950f013a88b8fd8cdc53b1eda1d4b73f9d9dc12546c8c87d68ff1435a0f8a006"
	prikeyA = "b5affb30846a00ef5aa39b57f913d70cd8cf6badd587239863cb67feacf6b9f30c34e800"
	pubkeyB = "881504d84a0659e14dcba59f24a98e71cda55b139615342668840c64678f1514941bbd053c7492fb9b719e6050cfa972efa491b79e11a1713824dda5f638fc0d9fa1b68be3c0f905"
	prikeyB = "b89c1ec0fb5b192c8bb8f6fcf9a871e4a67ef462f40d2b8ff426da1d1eaedd9696dc9d00"
	pubkeyC = "252fc7bc9a993b68dd7b13a00213c9cf4befe80da49940c52220f93c7147771ba2d783045cf0fbf2a86b32a62848befb96c0f38c0487a5ccc806ff28bb06d9faf803b93dda107003"
	prikeyC = "3e05de562a27fb6e25ac23ff8bcaa1ada0c253fa8ff7c6d15308f65d06b6990f64ee9601"
)

func main() {
	// target address for rpc connection. Default is "127.0.0.1"
	var addr string
	// number of transfer injections. Default is 50
	var transferNum int
	// number of vote injections. Default is 50
	var voteNum int
	// sleeping period between every two consecutive action injections in seconds. Default is 5
	var interval int
	flag.StringVar(&addr, "address", "127.0.0.1", "target address for rpc connection")
	flag.IntVar(&transferNum, "transfer-num", 50, "number of transfer injections")
	flag.IntVar(&voteNum, "vote-num", 50, "number of vote injections")
	flag.IntVar(&interval, "interval", 5, "sleep interval of two consecutively injected actions in seconds")
	flag.Parse()
	conn, err := grpc.Dial(addr+grpcPort, grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client := pb.NewChainServiceClient(conn)
	var ctx context.Context
	var cancel context.CancelFunc
	if interval == 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	} else {
		ctx, cancel = context.WithTimeout(context.Background(), time.Second*time.Duration(interval*(transferNum+voteNum)))
	}
	defer cancel()

	proxy := explorer.NewExplorerProxy("http://" + addr + jrpcPort)
	sender := constructAddress(pubkeyMiner, prikeyMiner)
	recipientA := constructAddress(pubkeyA, prikeyA)
	recipientB := constructAddress(pubkeyB, prikeyB)
	recipientC := constructAddress(pubkeyC, prikeyC)
	recipients := []*iotxaddress.Address{recipientA, recipientB, recipientC}
	rand.Seed(time.Now().UnixNano())

	addrDetails, err := proxy.GetAddressDetails(sender.RawAddress)
	if err != nil {
		panic(err)
	}
	i := addrDetails.Nonce + 1
	for ; transferNum > 0 && voteNum > 0; i += 2 {
		injectTransfer(ctx, client, sender, recipients[rand.Intn(3)], uint64(i))
		time.Sleep(time.Second * time.Duration(interval))
		injectVote(ctx, client, sender, recipients[rand.Intn(3)], uint64(i+1))
		time.Sleep(time.Second * time.Duration(interval))
		transferNum--
		voteNum--
	}
	switch {
	case transferNum > 0:
		for ; transferNum > 0; i++ {
			injectTransfer(ctx, client, sender, recipients[rand.Intn(3)], uint64(i))
			time.Sleep(time.Second * time.Duration(interval))
			transferNum--
		}
	case voteNum > 0:
		for ; voteNum > 0; i++ {
			injectVote(ctx, client, sender, recipients[rand.Intn(3)], uint64(i))
			time.Sleep(time.Second * time.Duration(interval))
			voteNum--
		}
	}
}

func injectTransfer(ctx context.Context, c pb.ChainServiceClient, sender *iotxaddress.Address, recipient *iotxaddress.Address, nonce uint64) {
	amount := uint64(0)
	for amount == uint64(0) {
		amount = uint64(rand.Intn(10))
	}
	fmt.Printf("Sending %v coins from 'miner'\n", amount)

	a := int64(amount)
	r, err := c.CreateRawTransfer(ctx, &pb.CreateRawTransferRequest{
		Sender: sender.RawAddress, Recipient: recipient.RawAddress, Amount: big.NewInt(a).Bytes(), Nonce: nonce, Data: []byte{}})
	if err != nil {
		panic(err)
	}
	fmt.Println("Created raw transfer")

	tsf := &pb.TransferPb{}
	if err := proto.Unmarshal(r.SerializedTransfer, tsf); err != nil {
		panic(err)
	}

	// Sign Transfer
	value := big.NewInt(0)
	transfer := action.NewTransfer(tsf.Nonce, value.SetBytes(tsf.Amount), tsf.Sender, tsf.Recipient)
	transfer, err = transfer.Sign(sender)
	if err != nil {
		panic(err)
	}
	tsf.SenderPubKey = transfer.SenderPublicKey
	tsf.Signature = transfer.Signature

	stsf, err := proto.Marshal(tsf)
	if err != nil {
		panic(err)
	}
	_, err = c.SendTransfer(ctx, &pb.SendTransferRequest{SerializedTransfer: stsf})
	if err != nil {
		panic(err)
	}
	fmt.Println("Sent out the signed transfer: ")

	fmt.Println("Version: ", tsf.Version)
	fmt.Println("Nonce: ", tsf.Nonce)
	fmt.Println("Amount: ", tsf.Amount)
	fmt.Println("Sender: ", tsf.Sender)
	fmt.Println("Recipient: ", tsf.Recipient)
	fmt.Println("Payload: ", tsf.Payload)
	fmt.Println("Sender Public Key: ", tsf.SenderPubKey)
	fmt.Println("Signature: ", tsf.Signature)
}

func injectVote(ctx context.Context, c pb.ChainServiceClient, sender *iotxaddress.Address, recipient *iotxaddress.Address, nonce uint64) {
	fmt.Println("Voting from 'miner'")
	r, err := c.CreateRawVote(ctx, &pb.CreateRawVoteRequest{Voter: sender.PublicKey, Votee: recipient.PublicKey, Nonce: nonce})
	if err != nil {
		panic(err)
	}
	fmt.Println("Created raw vote")

	votePb := &pb.VotePb{}
	if err := proto.Unmarshal(r.SerializedVote, votePb); err != nil {
		panic(err)
	}

	// Sign Vote
	vote := action.NewVote(votePb.Nonce, votePb.SelfPubkey, votePb.VotePubkey)
	vote, err = vote.Sign(sender)
	if err != nil {
		panic(err)
	}
	votePb.Signature = vote.Signature

	svote, err := proto.Marshal(votePb)
	if err != nil {
		panic(err)
	}
	_, err = c.SendVote(ctx, &pb.SendVoteRequest{SerializedVote: svote})
	if err != nil {
		panic(err)
	}
	fmt.Println("Sent out the signed vote: ")

	fmt.Println("Version: ", votePb.Version)
	fmt.Println("Nonce: ", votePb.Nonce)
	fmt.Println("Sender Public Key: ", votePb.SelfPubkey)
	fmt.Println("Recipient Public Key: ", votePb.VotePubkey)
	fmt.Println("Signature: ", votePb.Signature)
}

func constructAddress(pubkey, prikey string) *iotxaddress.Address {
	pubk, err := hex.DecodeString(pubkey)
	if err != nil {
		panic(err)
	}
	prik, err := hex.DecodeString(prikey)
	if err != nil {
		panic(err)
	}
	addr, err := iotxaddress.GetAddress(pubk, iotxaddress.IsTestnet, iotxaddress.ChainID)
	if err != nil {
		panic(err)
	}
	addr.PrivateKey = prik
	return addr
}
