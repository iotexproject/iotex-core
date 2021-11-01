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

type ServerV2 struct {
	core       *CoreService
	grpcServer *GrpcServer
	httpServer *HttpServer
}

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
	coreAPI, err := NewCoreService(cfg, chain, bs, sf, dao, indexer, bfIndexer, actPool, registry, opts...)
	if err != nil {
		return nil, err
	}
	return &ServerV2{
		core:       coreAPI,
		grpcServer: NewGRPCServer(coreAPI, cfg.API.Port),
		// httpServer: NewHTTPServer(coreAPI, cfg.API.Web3Port),
	}, nil
}

func (svr *ServerV2) Start() error {
	if err := svr.core.Start(); err != nil {
		return err
	}
	if err := svr.grpcServer.Start(); err != nil {
		return err
	}
	svr.httpServer.Start()
	return nil
}

func (svr *ServerV2) Stop() error {
	if err := svr.httpServer.Stop(); err != nil {
		return err
	}
	svr.grpcServer.Stop()
	if err := svr.core.Stop(); err != nil {
		return err
	}
	return nil
}
