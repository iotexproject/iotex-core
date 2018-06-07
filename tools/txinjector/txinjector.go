// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// This is a testing tool to inject fake transactions to the blockchain
// To use, run "make build" and " ./bin/txinjector"

package main

import (
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pb "github.com/iotexproject/iotex-core/proto"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/txvm"
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
		Sender: ta.Addrinfo["miner"].RawAddress, Recipient: ta.Addrinfo["alfa"].RawAddress, Amount: big.NewInt(a).Bytes()})
	if err != nil {
		panic(err)
	} else {
		fmt.Println("Created raw tx")
	}

	tx := &pb.TxPb{}
	if err := proto.Unmarshal(r.SerializedTransfer, tx); err != nil {
		panic(err)
	}

	for i, rin := range tx.TxIn {
		unlock, err := txvm.SignatureScript(rin.UnlockScript, ta.Addrinfo["miner"].PublicKey, ta.Addrinfo["miner"].PrivateKey)
		if err != nil {
			panic(err)
		}
		tx.TxIn[i].UnlockScript = unlock
		tx.TxIn[i].UnlockScriptSize = uint32(len(unlock))
	}

	stx, err := proto.Marshal(tx)
	if err != nil {
		panic(err)
	}
	_, err = c.SendTx(ctx, &pb.SendTransferRequest{SerializedTransfer: stx})
	if err != nil {
		panic(err)
	}
	fmt.Println("Sent out the signed tx: ")

	fmt.Println("version: ", tx.Version)
	for i := 0; i < len(tx.TxIn); i++ {
		fmt.Printf("txIn[%v]:\n", i)
		fmt.Printf("\thash: 0x%x\n", tx.TxIn[i].TxHash)
		fmt.Println("\tout index:", tx.TxIn[i].OutIndex)
		fmt.Println("\tunlock script:", tx.TxIn[i].UnlockScript)
		fmt.Println("\tunlock script size:", tx.TxIn[i].UnlockScriptSize)
		fmt.Println("\tsequence:", tx.TxIn[i].Sequence)
	}

	for i := 0; i < len(tx.TxOut); i++ {
		fmt.Printf("txOut[%v]:\n", i)
		fmt.Println("\tvalue:", tx.TxOut[i].Value)
		fmt.Println("\tlock script:", tx.TxOut[i].LockScript)
		fmt.Println("\tlock script size:", tx.TxOut[i].LockScriptSize)
	}
}
