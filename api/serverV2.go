package api

import (
	"context"
	"encoding/hex"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-election/committee"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action"
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

// TODO: remove internal call GetActionByActionHash() and GetReceiptByAction()

// GetActionByActionHash returns action by action hash
func (svr *ServerV2) GetActionByActionHash(h hash.Hash256) (action.SealedEnvelope, error) {
	if !svr.core.hasActionIndex || svr.core.indexer == nil {
		return action.SealedEnvelope{}, status.Error(codes.NotFound, blockindex.ErrActionIndexNA.Error())
	}

	selp, _, _, _, err := svr.core.getActionByActionHash(h)
	return selp, err
}

// GetReceiptByAction gets receipt with corresponding action hash
func (svr *ServerV2) GetReceiptByAction(h string) (*action.Receipt, string, error) {
	if !svr.core.hasActionIndex || svr.core.indexer == nil {
		return nil, "", status.Error(codes.NotFound, blockindex.ErrActionIndexNA.Error())
	}
	actHash, err := hash.HexStringToHash256(h)
	if err != nil {
		return nil, "", status.Error(codes.InvalidArgument, err.Error())
	}
	receipt, err := svr.core.ReceiptByActionHash(actHash)
	if err != nil {
		return nil, "", status.Error(codes.NotFound, err.Error())
	}
	blkHash, err := svr.core.getBlockHashByActionHash(actHash)
	if err != nil {
		return nil, "", status.Error(codes.NotFound, err.Error())
	}
	return receipt, hex.EncodeToString(blkHash[:]), nil
}
