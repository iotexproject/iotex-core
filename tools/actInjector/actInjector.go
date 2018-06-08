// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// This is a testing tool to inject fake transactions to the blockchain
// To use, run "make build" and " ./bin/actInjector"

package main

import (
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/crypto"
	pb "github.com/iotexproject/iotex-core/proto"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
)

const (
	port = ":42124"
)

func main() {
	conn, err := grpc.Dial("127.0.0.1"+port, grpc.WithInsecure())
	defer conn.Close()
	if err != nil {
		panic(err)
	}

	c := pb.NewChainServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	rand.Seed(time.Now().UnixNano())
	amount := uint64(0)
	for amount == uint64(0) {
		amount = uint64(rand.Intn(10))
	}
	fmt.Printf("Sending %v coins from 'miner' to 'alfa'", amount)

	a := int64(amount)
	r, err := c.CreateRawTx(ctx, &pb.CreateRawTransferRequest{
		Sender: ta.Addrinfo["miner"].RawAddress, Recipient: ta.Addrinfo["alfa"].RawAddress, Amount: big.NewInt(a).Bytes(), Nonce: uint64(1), Data: []byte{}})
	if err != nil {
		panic(err)
	} else {
		fmt.Println("Created raw transfer")
	}

	tsf := &pb.TransferPb{}
	if err := proto.Unmarshal(r.SerializedTransfer, tsf); err != nil {
		panic(err)
	}

	// Sign Transfer
	if tsf.Signature = crypto.Sign(ta.Addrinfo["miner"].PrivateKey, []byte{0x11, 0x22, 0x33, 0x44}); tsf.Signature == nil {
		panic(err)
	}

	// TODO: Should handle transfer as an action type
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
	fmt.Println("Lock Time: ", tsf.LockTime)
	fmt.Println("Nonce: ", tsf.Nonce)
	fmt.Println("Amount: ", tsf.Amount)
	fmt.Println("Sender: ", tsf.Sender)
	fmt.Println("Recipient: ", tsf.Recipient)
	fmt.Println("Payload: ", tsf.Payload)
	fmt.Println("Sender Public Key: ", tsf.SenderPubKey)
	fmt.Println("Signature: ", tsf.Signature)
}
