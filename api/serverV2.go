package api

import (
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
	grpcServer *GrpcServer
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
		grpcServer: NewGRPCServer(coreAPI, cfg.API.Port),
	}, nil
}

// Start starts the CoreService and the GRPC server
func (svr *ServerV2) Start() error {
	if err := svr.core.Start(); err != nil {
		return err
	}
	if err := svr.grpcServer.Start(); err != nil {
		return err
	}
	return nil
}

// Stop stops the GRPC server and the CoreService
func (svr *ServerV2) Stop() error {
	if err := svr.grpcServer.Stop(); err != nil {
		return err
	}
	if err := svr.core.Stop(); err != nil {
		return err
	}
	return nil
}
