// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rpcservice

import (
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/blockchain/trx"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/iotxaddress"
	pb "github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_dispatcher"
)

func decodeHash(in string) []byte {
	hash, _ := hex.DecodeString(in)
	return hash
}

func testingTx() *trx.Tx {
	txHash := common.ZeroHash32B
	copy(txHash[:], decodeHash("9de6306b08158c423330f7a27243a1a5cbe39bfd764f07818437882d21241567"))
	txIn1_0 := trx.NewTxInput(
		txHash, 0,
		decodeHash("40f9ea2b1357dde55519246a6ad82c466b9f2b988ff81a7c2fb114c932d44f322ba2edd178c2326739638b536e5f803977c24332b8f5b8ebc5f6683ff2bcaad90720b9b8d7316705dc4ff62bb323e610f3f5072abedc9834e999d6537f6681284ea2"),
		0)
	txOut1_0 := trx.NewTxOutput(10, 0)
	txOut1_0.LockScriptSize = 25
	txOut1_0.LockScript = decodeHash("65b014a97ce8e76ade9b3181c63432a62330a5ca83ab9ba1b1")
	txOut1_1 := trx.NewTxOutput(1, 1)
	txOut1_1.LockScriptSize = 25
	txOut1_1.LockScript = decodeHash("65b014af33097c8fd571c6c1efc52b0a802514ea0fbb03a1b1")
	txOut1_2 := trx.NewTxOutput(1, 2)
	txOut1_2.LockScriptSize = 25
	txOut1_2.LockScript = decodeHash("65b0140fb02223c1a78c3f1fb81a1572e8b07adb700bffa1b1")
	txOut1_3 := trx.NewTxOutput(1, 3)
	txOut1_3.LockScriptSize = 25
	txOut1_3.LockScript = decodeHash("65b01443251ba4fd765a2cfa65256aabd64f98c5c00e40a1b1")
	txOut1_4 := trx.NewTxOutput(1, 4)
	txOut1_4.LockScriptSize = 25
	txOut1_4.LockScript = decodeHash("65b01430f1db72a44136e8634121b6730c2b8ef094f1c9a1b1")
	txOut1_5 := trx.NewTxOutput(5, 5)
	txOut1_5.LockScriptSize = 25
	txOut1_5.LockScript = decodeHash("65b014d94ee6c7205e85c3d97c557f08faf8ac41102806a1b1")
	txOut1_6 := trx.NewTxOutput(9999999981, 6)
	txOut1_6.LockScriptSize = 25
	txOut1_6.LockScript = decodeHash("65b014d4f743a24d5386f8d1c2a648da7015f08800cd11a1b1")
	return &trx.Tx{
		Version:  1,
		TxIn:     []*trx.TxInput{txIn1_0},
		TxOut:    []*trx.TxOutput{txOut1_0, txOut1_1, txOut1_2, txOut1_3, txOut1_4, txOut1_5, txOut1_6},
		LockTime: 0,
	}
}

func testingTransfer() *action.Transfer {
	sender, _ := iotxaddress.NewAddress(true, []byte{0x00, 0x00, 0x00, 0x01})
	recipient, _ := iotxaddress.NewAddress(true, []byte{0x00, 0x00, 0x00, 0x01})
	return action.NewTransfer(uint64(1), big.NewInt(100), sender.RawAddress, recipient.RawAddress)
}

func TestCreateRawTransfer(t *testing.T) {
	cfg := config.Config{
		RPC: config.RPC{
			Port: ":42124",
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mbc := mock_blockchain.NewMockBlockchain(ctrl)
	mdp := mock_dispatcher.NewMockDispatcher(ctrl)

	cbinvoked := false
	bcb := func(msg proto.Message) error {
		cbinvoked = true
		return nil
	}

	s := NewChainServer(cfg.RPC, mbc, mdp, bcb)
	assert.NotNil(t, s)
	s.Start()
	defer s.Stop()

	// Set up a connection to the server.
	conn, err := grpc.Dial("127.0.0.1:42124", grpc.WithInsecure())
	assert.Nil(t, err)
	defer conn.Close()

	// Contact the server and print out its response.
	c := pb.NewChainServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	mbc.EXPECT().CreateRawTransfer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testingTransfer()).Times(1)
	mdp.EXPECT().HandleBroadcast(gomock.Any(), gomock.Any()).Times(0)
	r, err := c.CreateRawTx(ctx, &pb.CreateRawTransferRequest{Sender: "Alice", Recipient: "Bob", Amount: big.NewInt(int64(100)).Bytes()})
	assert.Nil(t, err)
	assert.Equal(t, 113, len(r.SerializedTransfer))
	assert.False(t, cbinvoked)
}

func TestSendTx(t *testing.T) {
	cfg := config.Config{
		RPC: config.RPC{
			Port: ":42124",
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mbc := mock_blockchain.NewMockBlockchain(ctrl)
	mdp := mock_dispatcher.NewMockDispatcher(ctrl)

	cbinvoked := false
	bcb := func(msg proto.Message) error {
		cbinvoked = true
		return nil
	}

	s := NewChainServer(cfg.RPC, mbc, mdp, bcb)
	assert.NotNil(t, s)
	s.Start()
	defer s.Stop()

	// Set up a connection to the server.
	conn, err := grpc.Dial("127.0.0.1:42124", grpc.WithInsecure())
	assert.Nil(t, err)
	defer conn.Close()

	// Contact the server and print out its response.
	c := pb.NewChainServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stx, err := proto.Marshal(testingTx().ConvertToTxPb())
	assert.Nil(t, err)

	mdp.EXPECT().HandleBroadcast(gomock.Any(), gomock.Any()).Times(1)
	_, err = c.SendTx(ctx, &pb.SendTransferRequest{SerializedTransfer: stx})
	assert.Nil(t, err)
	assert.True(t, cbinvoked)
}
