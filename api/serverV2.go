package api

import (
	"context"

	"github.com/iotexproject/iotex-election/committee"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/state/factory"
)

// ServerV2 provides api for user to interact with blockchain data
type ServerV2 struct {
	core       *coreService
	GrpcServer *GRPCServer
	web3Server *Web3Server
}

// Config represents the config to setup api
type Config struct {
	broadcastHandler  BroadcastOutbound
	electionCommittee committee.Committee
}

// Option is the option to override the api config
type Option func(cfg *Config) error

// BroadcastOutbound sends a broadcast message to the whole network
type BroadcastOutbound func(ctx context.Context, chainID uint32, msg proto.Message) error

// WithBroadcastOutbound is the option to broadcast msg outbound
func WithBroadcastOutbound(broadcastHandler BroadcastOutbound) Option {
	return func(cfg *Config) error {
		cfg.broadcastHandler = broadcastHandler
		return nil
	}
}

// WithNativeElection is the option to return native election data through API.
func WithNativeElection(committee committee.Committee) Option {
	return func(cfg *Config) error {
		cfg.electionCommittee = committee
		return nil
	}
}

// NewServerV2 creates a new server with coreService and GRPC Server
func NewServerV2(
	cfg config.Config,
	chain blockchain.Blockchain,
	bs blocksync.BlockSync,
	sf factory.Factory,
	dao blockdao.BlockDAO,
	indexer blockindex.Indexer,
	bfIndexer blockindex.BloomFilterIndexer,
	actPool actpool.ActPool,
	registry *protocol.Registry,
	opts ...Option,
) (*ServerV2, error) {
	coreAPI, err := newCoreService(cfg, chain, bs, sf, dao, indexer, bfIndexer, actPool, registry, opts...)
	if err != nil {
		return nil, err
	}
	return &ServerV2{
		core:       coreAPI,
		GrpcServer: NewGRPCServer(coreAPI, cfg.API.Port),
		web3Server: NewWeb3Server(coreAPI, cfg.API.Web3Port),
	}, nil
}

// Start starts the CoreService and the GRPC server
func (svr *ServerV2) Start() error {
	if err := svr.core.Start(); err != nil {
		return err
	}
	if err := svr.GrpcServer.Start(); err != nil {
		return err
	}
	svr.web3Server.Start()
	return nil
}

// Stop stops the GRPC server and the CoreService
func (svr *ServerV2) Stop() error {
	if err := svr.web3Server.Stop(); err != nil {
		return err
	}
	if err := svr.GrpcServer.Stop(); err != nil {
		return err
	}
	if err := svr.core.Stop(); err != nil {
		return err
	}
	return nil
}
