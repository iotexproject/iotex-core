// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"

	"github.com/pkg/errors"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/tracer"
	"github.com/iotexproject/iotex-core/state/factory"
)

// ServerV2 provides api for user to interact with blockchain data
type ServerV2 struct {
	core         CoreService
	grpcServer   *GRPCServer
	httpSvr      *HTTPServer
	websocketSvr *HTTPServer
	tracer       *tracesdk.TracerProvider
}

// NewServerV2 creates a new server with coreService and GRPC Server
func NewServerV2(
	cfg config.API,
	scheme string,
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
	coreAPI, err := newCoreService(cfg,scheme, chain, bs, sf, dao, indexer, bfIndexer, actPool, registry, opts...)
	if err != nil {
		return nil, err
	}
	web3Handler := NewWeb3Handler(coreAPI, cfg.RedisCacheURL)

	tp, err := tracer.NewProvider(
		tracer.WithServiceName(cfg.Tracer.ServiceName),
		tracer.WithEndpoint(cfg.Tracer.EndPoint),
		tracer.WithInstanceID(cfg.Tracer.InstanceID),
		tracer.WithSamplingRatio(cfg.Tracer.SamplingRatio),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot config tracer provider")
	}

	return &ServerV2{
		core:         coreAPI,
		grpcServer:   NewGRPCServer(coreAPI, cfg.GRPCPort),
		httpSvr:      NewHTTPServer("", cfg.HTTPPort, newHTTPHandler(web3Handler)),
		websocketSvr: NewHTTPServer("", cfg.WebSocketPort, NewWebsocketHandler(web3Handler)),
		tracer:       tp,
	}, nil
}

// Start starts the CoreService and the GRPC server
func (svr *ServerV2) Start(ctx context.Context) error {
	if err := svr.core.Start(ctx); err != nil {
		return err
	}
	if svr.grpcServer != nil {
		if err := svr.grpcServer.Start(ctx); err != nil {
			return err
		}
	}
	if svr.httpSvr != nil {
		if err := svr.httpSvr.Start(ctx); err != nil {
			return err
		}
	}
	if svr.websocketSvr != nil {
		if err := svr.websocketSvr.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Stop stops the GRPC server and the CoreService
func (svr *ServerV2) Stop(ctx context.Context) error {
	if svr.tracer != nil {
		if err := svr.tracer.Shutdown(context.Background()); err != nil {
			return errors.Wrap(err, "failed to shutdown api tracer")
		}
	}
	if svr.websocketSvr != nil {
		if err := svr.websocketSvr.Stop(ctx); err != nil {
			return err
		}
	}
	if svr.httpSvr != nil {
		if err := svr.httpSvr.Stop(ctx); err != nil {
			return err
		}
	}
	if svr.grpcServer != nil {
		if err := svr.grpcServer.Stop(ctx); err != nil {
			return err
		}
	}
	if err := svr.core.Stop(ctx); err != nil {
		return err
	}
	return nil
}

// ReceiveBlock receives the new block
func (svr *ServerV2) ReceiveBlock(blk *block.Block) error {
	return svr.core.ReceiveBlock(blk)
}

// CoreService returns the coreservice of the api
func (svr *ServerV2) CoreService() CoreService {
	return svr.core
}
