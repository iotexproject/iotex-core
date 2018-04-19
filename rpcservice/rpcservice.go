// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rpcservice

import (
	"net"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/iotexproject/iotex-core/blockchain"
	cm "github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/iotxaddress"
	pb "github.com/iotexproject/iotex-core/proto"
)

// Chainserver is used to implement Chain Service
type Chainserver struct {
	blockchain  blockchain.IBlockchain
	config      config.RPC
	dispatcher  cm.Dispatcher
	grpcserver  *grpc.Server
	broadcastcb func(proto.Message) error
}

// NewChainServer creates an instance of chainserver
func NewChainServer(c config.RPC, b blockchain.IBlockchain, dp cm.Dispatcher, cb func(proto.Message) error) *Chainserver {
	if cb == nil {
		glog.Fatal("cannot new chain server with nil callback")
	}
	return &Chainserver{blockchain: b, config: c, dispatcher: dp, broadcastcb: cb}
}

// CreateRawTx creates a unsigned raw transaction
func (s *Chainserver) CreateRawTx(ctx context.Context, in *pb.CreateRawTxRequest) (*pb.CreateRawTxReply, error) {
	if len(in.From) == 0 || len(in.To) == 0 || in.Value == 0 {
		return nil, errors.New("invalid CreateRawTxRequest")
	}

	bal := s.blockchain.BalanceOf(in.From)
	if bal < in.Value {
		return nil, errors.New("not enough balance from address: " + in.From)
	}

	p := []*blockchain.Payee{{in.To, in.Value}}
	tx := s.blockchain.CreateRawTransaction(iotxaddress.Address{Address: in.From}, in.Value, p)
	stx, err := proto.Marshal(tx.ConvertToTxPb())
	if err != nil {
		return nil, err
	}
	return &pb.CreateRawTxReply{SerializedTx: stx}, nil
}

// SendTx sends out a signed raw transaction
func (s *Chainserver) SendTx(ctx context.Context, in *pb.SendTxRequest) (*pb.SendTxReply, error) {
	if len(in.SerializedTx) == 0 {
		return nil, errors.New("invalid SendTxRequest")
	}

	tx := &pb.TxPb{}
	if err := proto.Unmarshal(in.SerializedTx, tx); err != nil {
		return nil, err
	}
	// broadcast to the network
	if err := s.broadcastcb(tx); err != nil {
		return nil, err
	}
	// send to txpool via dispatcher
	s.dispatcher.HandleBroadcast(tx, nil)
	return &pb.SendTxReply{}, nil
}

// Start starts the chain server
func (s *Chainserver) Start() error {
	if s.config == (config.RPC{}) {
		glog.Warning("Chain service is not configured")
		return nil
	}

	lis, err := net.Listen("tcp", s.config.Port)
	if err != nil {
		glog.Fatalf("Chain server failed to listen: %v", err)
		return err
	}
	glog.Infof("Chain server is listening on %v", lis.Addr().String())

	s.grpcserver = grpc.NewServer()
	pb.RegisterChainServiceServer(s.grpcserver, s)
	reflection.Register(s.grpcserver)

	go func() {
		if err := s.grpcserver.Serve(lis); err != nil {
			glog.Fatalf("Node failed to serve: %v", err)
		}
	}()
	return nil
}

// Stop stops the chain server
func (s *Chainserver) Stop() error {
	s.grpcserver.Stop()
	return nil
}
