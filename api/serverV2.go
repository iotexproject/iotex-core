package api

import (
	"context"

	"github.com/iotexproject/iotex-election/committee"
	"github.com/pkg/errors"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/protobuf/proto"

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
	core       CoreService
	GrpcServer *GRPCServer
	web3Server *Web3Server
	tracer     *tracesdk.TracerProvider
}

// Config represents the config to setup api
type Config struct {
	broadcastHandler  BroadcastOutbound
	electionCommittee committee.Committee
	hasActionIndex    bool
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

// WithActionIndex is the option which enables action index related features
func WithActionIndex() Option {
	return func(cfg *Config) error {
		cfg.hasActionIndex = true
		return nil
	}
}

// NewServerV2 creates a new server with coreService and GRPC Server
func NewServerV2(
	cfg config.API,
	chain blockchain.Blockchain,
	bs blocksync.BlockSync,
	sf factory.Factory,
	dao blockdao.BlockDAO,
	indexer blockindex.Indexer,
	bfIndexer blockindex.BloomFilterIndexer,
	actPool actpool.ActPool,
	registry *protocol.Registry,
	workerNumbers int,
	opts ...Option,
) (*ServerV2, error) {
	coreAPI, err := newCoreService(cfg, chain, bs, sf, dao, indexer, bfIndexer, actPool, registry, workerNumbers, opts...)
	if err != nil {
		return nil, err
	}
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
		core:       coreAPI,
		GrpcServer: NewGRPCServer(coreAPI, cfg.Port),
		web3Server: NewWeb3Server(coreAPI, cfg.Web3Port, cfg.RedisCacheURL, cfg.RangeQueryLimit),
		tracer:     tp,
	}, nil
}

// Start starts the CoreService and the GRPC server
func (svr *ServerV2) Start(ctx context.Context) error {
	if err := svr.core.Start(ctx); err != nil {
		return err
	}
	if err := svr.GrpcServer.Start(ctx); err != nil {
		return err
	}
	svr.web3Server.Start(ctx)
	return nil
}

// Stop stops the GRPC server and the CoreService
func (svr *ServerV2) Stop(ctx context.Context) error {
	if svr.tracer != nil {
		if err := svr.tracer.Shutdown(context.Background()); err != nil {
			return errors.Wrap(err, "failed to shutdown api tracer")
		}
	}
	if err := svr.web3Server.Stop(ctx); err != nil {
		return err
	}
	if err := svr.GrpcServer.Stop(ctx); err != nil {
		return err
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
