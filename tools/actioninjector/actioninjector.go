// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// This is a testing tool to inject fake transactions to the blockchain
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

	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
	pb "github.com/iotexproject/iotex-core/proto"
)

const (
	port        = ":42124"
	pubkeyMiner = "b9b8d7316705dc4ff62bb323e610f3f5072abedc9834e999d6537f6681284ea2"
	prikeyMiner = "7fbb20b87d34eade61351165aa4c6fa5d87dd349368dd6b9034ea3d3e918c706b9b8d7316705dc4ff62bb323e610f3f5072abedc9834e999d6537f6681284ea2"
	pubkeyA     = "2c9ccbeb9ee91271f7e5c2103753be9c9edff847e1a51227df6a6b0765f31a4b424e84027b44a663950f013a88b8fd8cdc53b1eda1d4b73f9d9dc12546c8c87d68ff1435a0f8a006"
	prikeyA     = "b5affb30846a00ef5aa39b57f913d70cd8cf6badd587239863cb67feacf6b9f30c34e800"
)

func main() {
	var count int
	flag.IntVar(&count, "count", 10, "number of action injections")
	flag.Parse()
	conn, err := grpc.Dial("127.0.0.1"+port, grpc.WithInsecure())
	defer conn.Close()
	if err != nil {
		panic(err)
	}

	c := pb.NewChainServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(count*5))
	defer cancel()

	sender := constructAddress(pubkeyMiner, prikeyMiner)
	recipient := constructAddress(pubkeyA, prikeyA)
	for i := 1; i <= count; i++ {
		injectAction(ctx, c, sender, recipient, uint64(i))
		time.Sleep(time.Second * 5)
	}
}

func injectAction(ctx context.Context, c pb.ChainServiceClient, sender *iotxaddress.Address, recipient *iotxaddress.Address, nonce uint64) {
	rand.Seed(time.Now().UnixNano())
	amount := uint64(0)
	for amount == uint64(0) {
		amount = uint64(rand.Intn(10))
	}
	fmt.Printf("Sending %v coins from 'miner' to 'alfa'", amount)

	a := int64(amount)
	r, err := c.CreateRawTx(ctx, &pb.CreateRawTransferRequest{
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
	if tsf.Signature = crypto.Sign(sender.PrivateKey, []byte{0x11, 0x22, 0x33, 0x44}); tsf.Signature == nil {
		panic(err)
	}

	stsf, err := proto.Marshal(tsf)
	if err != nil {
		panic(err)
	}
	_, err = c.SendTx(ctx, &pb.SendTransferRequest{SerializedTransfer: stsf})
	if err != nil {
		panic(err)
	}
	fmt.Println("Sent out the signed tx: ")

	fmt.Println("Version: ", tsf.Version)
	fmt.Println("Nonce: ", tsf.Nonce)
	fmt.Println("Amount: ", tsf.Amount)
	fmt.Println("Sender: ", tsf.Sender)
	fmt.Println("Recipient: ", tsf.Recipient)
	fmt.Println("Payload: ", tsf.Payload)
	fmt.Println("Sender Public Key: ", tsf.SenderPubKey)
	fmt.Println("Signature: ", tsf.Signature)
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
	addr, err := iotxaddress.GetAddress(pubk, false, []byte{0x01, 0x02, 0x03, 0x04})
	if err != nil {
		panic(err)
	}
	addr.PrivateKey = prik
	return addr
}
