// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
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
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/iotxaddress"
	pb "github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_dispatcher"
)

func testingTransfer() *action.Transfer {
	sender, _ := iotxaddress.NewAddress(true, []byte{0x00, 0x00, 0x00, 0x01})
	recipient, _ := iotxaddress.NewAddress(true, []byte{0x00, 0x00, 0x00, 0x01})
	return action.NewTransfer(uint64(1), big.NewInt(100), sender.RawAddress, recipient.RawAddress)
}

func testingVote() *action.Vote {
	sender, _ := iotxaddress.NewAddress(true, []byte{0x00, 0x00, 0x00, 0x01})
	recipient, _ := iotxaddress.NewAddress(true, []byte{0x00, 0x00, 0x00, 0x01})
	return action.NewVote(uint64(1), sender.PublicKey, recipient.PublicKey)
}

func TestCreateRawTransfer(t *testing.T) {
	cfg := config.Config{
		RPC: config.RPC{
			Addr: "127.0.0.1:42124",
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
	r, err := c.CreateRawTransfer(ctx, &pb.CreateRawTransferRequest{Sender: "Alice", Recipient: "Bob", Amount: big.NewInt(int64(100)).Bytes()})
	assert.Nil(t, err)
	assert.Equal(t, 109, len(r.SerializedTransfer))
	assert.False(t, cbinvoked)
}

func TestCreateRawVote(t *testing.T) {
	cfg := config.Config{
		RPC: config.RPC{
			Addr: "127.0.0.1:42124",
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

	mbc.EXPECT().CreateRawVote(gomock.Any(), gomock.Any(), gomock.Any()).Return(testingVote()).Times(1)
	mdp.EXPECT().HandleBroadcast(gomock.Any(), gomock.Any()).Times(0)
	senderPubkey, _ := hex.DecodeString("336eb60a5741f585a8e81de64e071327a3b96c15af4af5723598a07b6121e8e813bbd0056ba71ae29c0d64252e913f60afaeb11059908b81ff27cbfa327fd371d35f5ec0cbc01705")
	recipientPubkey, _ := hex.DecodeString("2c9ccbeb9ee91271f7e5c2103753be9c9edff847e1a51227df6a6b0765f31a4b424e84027b44a663950f013a88b8fd8cdc53b1eda1d4b73f9d9dc12546c8c87d68ff1435a0f8a006")
	r, err := c.CreateRawVote(ctx, &pb.CreateRawVoteRequest{Voter: senderPubkey, Votee: recipientPubkey})
	assert.Nil(t, err)
	assert.Equal(t, 152, len(r.SerializedVote))
	assert.False(t, cbinvoked)
}

func TestSendTransfer(t *testing.T) {
	cfg := config.Config{
		RPC: config.RPC{
			Addr: "127.0.0.1:42124",
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

	stsf, err := proto.Marshal(testingTransfer().ConvertToTransferPb())
	assert.Nil(t, err)

	mdp.EXPECT().HandleBroadcast(gomock.Any(), gomock.Any()).Times(1)
	_, err = c.SendTransfer(ctx, &pb.SendTransferRequest{SerializedTransfer: stsf})
	assert.Nil(t, err)
	assert.True(t, cbinvoked)
}

func TestSendVote(t *testing.T) {
	cfg := config.Config{
		RPC: config.RPC{
			Addr: "127.0.0.1:42124",
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

	svote, err := proto.Marshal(testingVote().ConvertToVotePb())
	assert.Nil(t, err)

	mdp.EXPECT().HandleBroadcast(gomock.Any(), gomock.Any()).Times(1)
	_, err = c.SendVote(ctx, &pb.SendVoteRequest{SerializedVote: svote})
	assert.Nil(t, err)
	assert.True(t, cbinvoked)
}
