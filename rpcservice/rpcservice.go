// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
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
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/dispatch/dispatcher"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	pb "github.com/iotexproject/iotex-core/proto"
)

// Chainserver is used to implement Chain Service
type Chainserver struct {
	blockchain  blockchain.Blockchain
	config      config.RPC
	dispatcher  dispatcher.Dispatcher
	grpcserver  *grpc.Server
	broadcastcb func(proto.Message) error
}

// NewChainServer creates an instance of chainserver
func NewChainServer(c config.RPC, b blockchain.Blockchain, dp dispatcher.Dispatcher, cb func(proto.Message) error) *Chainserver {
	if cb == nil {
		logger.Error().Msg("cannot new chain server with nil callback")
		return nil
	}
	return &Chainserver{blockchain: b, config: c, dispatcher: dp, broadcastcb: cb}
}

// CreateRawTransfer creates an unsigned raw transaction
func (s *Chainserver) CreateRawTransfer(ctx context.Context, in *pb.CreateRawTransferRequest) (*pb.CreateRawTransferResponse, error) {
	logger.Debug().Msg("receive create raw transfer request")

	if len(in.Sender) == 0 || len(in.Recipient) == 0 {
		return nil, errors.New("invalid CreateRawTransferRequest")
	}
	amount := big.NewInt(0)
	amount.SetBytes(in.Amount)

	tsf := s.blockchain.CreateRawTransfer(in.Nonce, &iotxaddress.Address{RawAddress: in.Sender}, amount, &iotxaddress.Address{RawAddress: in.Recipient})
	stsf, err := proto.Marshal(tsf.ConvertToTransferPb())
	if err != nil {
		return nil, err
	}
	return &pb.CreateRawTransferResponse{SerializedTransfer: stsf}, nil
}

// SendTransfer sends out a signed raw transaction
func (s *Chainserver) SendTransfer(ctx context.Context, in *pb.SendTransferRequest) (*pb.SendTransferResponse, error) {
	logger.Debug().Msg("receive send transfer request")

	if len(in.SerializedTransfer) == 0 {
		return nil, errors.New("invalid SendTransferRequest")
	}

	tsf := &pb.TransferPb{}
	if err := proto.Unmarshal(in.SerializedTransfer, tsf); err != nil {
		return nil, err
	}
	// Wrap TransferPb as an ActionPb
	action := &pb.ActionPb{&pb.ActionPb_Transfer{tsf}}
	// broadcast to the network
	if err := s.broadcastcb(action); err != nil {
		return nil, err
	}
	// send to actpool via dispatcher
	s.dispatcher.HandleBroadcast(action, nil)
	return &pb.SendTransferResponse{}, nil
}

// CreateRawVote creates an unsigned raw vote
func (s *Chainserver) CreateRawVote(ctx context.Context, in *pb.CreateRawVoteRequest) (*pb.CreateRawVoteResponse, error) {
	if len(in.Voter) == 0 || len(in.Votee) == 0 {
		return nil, errors.New("invalid CreateRawVoteRequest")
	}

	vote := s.blockchain.CreateRawVote(in.Nonce, in.Voter, in.Votee)
	svote, err := proto.Marshal(vote.ConvertToVotePb())
	if err != nil {
		return nil, err
	}
	return &pb.CreateRawVoteResponse{SerializedVote: svote}, nil
}

// SendVote sends out a signed vote
func (s *Chainserver) SendVote(ctx context.Context, in *pb.SendVoteRequest) (*pb.SendVoteResponse, error) {
	if len(in.SerializedVote) == 0 {
		return nil, errors.New("invalid SendVoteRequest")
	}

	vote := &pb.VotePb{}
	if err := proto.Unmarshal(in.SerializedVote, vote); err != nil {
		return nil, err
	}
	// Wrap VotePb as an ActionPb
	action := &pb.ActionPb{&pb.ActionPb_Vote{vote}}
	// broadcast to the network
	if err := s.broadcastcb(action); err != nil {
		return nil, err
	}
	// send to actpool via dispatcher
	s.dispatcher.HandleBroadcast(action, nil)
	return &pb.SendVoteResponse{}, nil
}

// Start starts the chain server
func (s *Chainserver) Start() error {
	if s.config == (config.RPC{}) {
		logger.Warn().Msg("Chain service is not configured")
		return nil
	}

	lis, err := net.Listen("tcp", s.config.Addr)
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
