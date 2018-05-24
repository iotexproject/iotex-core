// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rpcservice

import (
	"math/big"
	"net"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/iotexproject/iotex-core/blockchain"
	cm "github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	pb "github.com/iotexproject/iotex-core/proto"
)

// Chainserver is used to implement Chain Service
type Chainserver struct {
	blockchain  blockchain.Blockchain
	config      config.RPC
	dispatcher  cm.Dispatcher
	grpcserver  *grpc.Server
	broadcastcb func(proto.Message) error
}

// NewChainServer creates an instance of chainserver
func NewChainServer(c config.RPC, b blockchain.Blockchain, dp cm.Dispatcher, cb func(proto.Message) error) *Chainserver {
	if cb == nil {
		logger.Error().Msg("cannot new chain server with nil callback")
		return nil
	}
	return &Chainserver{blockchain: b, config: c, dispatcher: dp, broadcastcb: cb}
}

// CreateRawTx creates a unsigned raw transaction
func (s *Chainserver) CreateRawTx(ctx context.Context, in *pb.CreateRawTxRequest) (*pb.CreateRawTxReply, error) {
	if len(in.From) == 0 || len(in.To) == 0 || in.Value == 0 {
		return nil, errors.New("invalid CreateRawTxRequest")
	}

	bal := s.blockchain.BalanceOf(in.From)
	tmp := big.NewInt(0)
	if bal.Cmp(tmp.SetUint64(in.Value)) == -1 {
		return nil, errors.New("not enough balance from address: " + in.From)
	}

	p := []*blockchain.Payee{{in.To, in.Value}}
	tx := s.blockchain.CreateRawTransaction(&iotxaddress.Address{RawAddress: in.From}, in.Value, p)
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
		logger.Warn().Msg("Chain service is not configured")
		return nil
	}

	lis, err := net.Listen("tcp", s.config.Port)
	if err != nil {
		logger.Error().Err(err).Msg("Chain server failed to listen")
		return err
	}
	logger.Info().
		Str("addr", lis.Addr().String()).
		Msg("Chain server is listening")

	s.grpcserver = grpc.NewServer()
	pb.RegisterChainServiceServer(s.grpcserver, s)
	reflection.Register(s.grpcserver)

	go func() {
		if err := s.grpcserver.Serve(lis); err != nil {
			logger.Fatal().Err(err).Msg("Node failed to serve")
		}
	}()
	return nil
}

// Stop stops the chain server
func (s *Chainserver) Stop() error {
	s.grpcserver.Stop()
	return nil
}
