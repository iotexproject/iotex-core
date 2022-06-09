// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"encoding/hex"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-election/committee"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/infra/action"
	"github.com/iotexproject/iotex-core/infra/action/protocol"
	"github.com/iotexproject/iotex-core/infra/actpool"
	"github.com/iotexproject/iotex-core/infra/blockchain"
	"github.com/iotexproject/iotex-core/pkg/log"
)

const (
	_workerNumbers = 5
)

type (
	// CoreService provides api interface for user to interact with blockchain data
	CoreService interface {
		// Start starts the API server
		Start(ctx context.Context) error
		// Stop stops the API server
		Stop(ctx context.Context) error
		// SendAction is the API to send an action to blockchain.
		SendAction(ctx context.Context, in *iotextypes.Action) (string, error)
	}

	// coreService implements the CoreService interface
	coreService struct {
		bc                blockchain.Blockchain
		// bs                blocksync.BlockSync
		// sf                factory.Factory
		// dao               blockdao.BlockDAO
		// indexer           blockindex.Indexer
		// bfIndexer         blockindex.BloomFilterIndexer
		ap actpool.ActPool
		// gs                *gasstation.GasStation
		broadcastHandler BroadcastOutbound
		cfg              config.API
		registry         *protocol.Registry
		// chainListener     apitypes.Listener
		hasActionIndex    bool
		electionCommittee committee.Committee
		readCache         *ReadCache
	}

	// jobDesc provides a struct to get and store logs in core.LogsInRange
	jobDesc struct {
		idx    int
		blkNum uint64
	}
)

type intrinsicGasCalculator interface {
	IntrinsicGas() (uint64, error)
}

var (
	// ErrNotFound indicates the record isn't found
	ErrNotFound = errors.New("not found")
)

// Option is the option to override the api config
type Option func(cfg *Config) error

// BroadcastOutbound sends a broadcast message to the whole network
type BroadcastOutbound func(ctx context.Context, chainID uint32, msg proto.Message) error

// Config represents the config to setup api
type Config struct {
	broadcastHandler  BroadcastOutbound
	electionCommittee committee.Committee
	hasActionIndex    bool
}

// newcoreService creates a api server that contains major blockchain components
// func newCoreService(
// 	cfg config.API,
// 	chain blockchain.Blockchain,
// 	bs blocksync.BlockSync,
// 	sf factory.Factory,
// 	dao blockdao.BlockDAO,
// 	indexer blockindex.Indexer,
// 	bfIndexer blockindex.BloomFilterIndexer,
// 	actPool actpool.ActPool,
// 	registry *protocol.Registry,
// 	opts ...Option,
// ) (CoreService, error) {
// 	apiCfg := Config{}
// 	for _, opt := range opts {
// 		if err := opt(&apiCfg); err != nil {
// 			return nil, err
// 		}
// 	}

// 	if cfg == (config.API{}) {
// 		log.L().Warn("API server is not configured.")
// 		cfg = config.Default.API
// 	}

// 	if cfg.RangeQueryLimit < uint64(cfg.TpsWindow) {
// 		return nil, errors.New("range query upper limit cannot be less than tps window")
// 	}
// 	return &coreService{
// 		bc:                chain,
// 		bs:                bs,
// 		sf:                sf,
// 		dao:               dao,
// 		indexer:           indexer,
// 		bfIndexer:         bfIndexer,
// 		ap:                actPool,
// 		broadcastHandler:  apiCfg.broadcastHandler,
// 		cfg:               cfg,
// 		registry:          registry,
// 		chainListener:     NewChainListener(500),
// 		gs:                gasstation.NewGasStation(chain, dao, cfg),
// 		electionCommittee: apiCfg.electionCommittee,
// 		readCache:         NewReadCache(),
// 		hasActionIndex:    apiCfg.hasActionIndex,
// 	}, nil
// }

// Start starts the API server
func (core *coreService) Start(_ context.Context) error {
	// if err := core.chainListener.Start(); err != nil {
	// 	return errors.Wrap(err, "failed to start blockchain listener")
	// }
	return nil
}

// Stop stops the API server
func (core *coreService) Stop(_ context.Context) error {
	// return core.chainListener.Stop()
	return nil
}

// SendAction is the API to send an action to blockchain.
func (core *coreService) SendAction(ctx context.Context, in *iotextypes.Action) (string, error) {
	log.Logger("api").Debug("receive send action request")
	selp, err := (&action.Deserializer{}).ActionToSealedEnvelope(in)
	if err != nil {
		return "", status.Error(codes.InvalidArgument, err.Error())
	}

	// reject action if chainID is not matched at KamchatkaHeight
	if err := core.validateChainID(in.GetCore().GetChainID()); err != nil {
		return "", err
	}

	// Add to local actpool
	ctx = protocol.WithRegistry(ctx, core.registry)
	hash, err := selp.Hash()
	if err != nil {
		return "", err
	}
	l := log.Logger("api").With(zap.String("actionHash", hex.EncodeToString(hash[:])))
	if err = core.ap.Add(ctx, selp); err != nil {
		txBytes, serErr := proto.Marshal(in)
		if serErr != nil {
			l.Error("Data corruption", zap.Error(serErr))
		} else {
			l.With(zap.String("txBytes", hex.EncodeToString(txBytes))).Error("Failed to accept action", zap.Error(err))
		}
		st := status.New(codes.Internal, err.Error())
		br := &errdetails.BadRequest{
			FieldViolations: []*errdetails.BadRequest_FieldViolation{
				{
					Field:       "Action rejected",
					Description: action.LoadErrorDescription(err),
				},
			},
		}
		st, err := st.WithDetails(br)
		if err != nil {
			log.Logger("api").Panic("Unexpected error attaching metadata", zap.Error(err))
		}
		return "", st.Err()
	}
	// If there is no error putting into local actpool,
	// Broadcast it to the network
	if err = core.broadcastHandler(ctx, core.bc.ChainID(), in); err != nil {
		l.Warn("Failed to broadcast SendAction request.", zap.Error(err))
	}
	return hex.EncodeToString(hash[:]), nil
}

func (core *coreService) validateChainID(chainID uint32) error {
	if ge := core.bc.Genesis(); ge.IsMidway(core.bc.TipHeight()) && chainID != core.bc.ChainID() && chainID != 0 {
		return status.Errorf(codes.InvalidArgument, "ChainID does not match, expecting %d, got %d", core.bc.ChainID(), chainID)
	}
	return nil
}
