// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package chainservice

import (
	"context"
	"math/big"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/api"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-election/committee"
)

// ChainService is a blockchain service with all blockchain components.
type ChainService struct {
	actpool           actpool.ActPool
	blocksync         blocksync.BlockSync
	consensus         consensus.Consensus
	chain             blockchain.Blockchain
	electionCommittee committee.Committee
	rDPoSProtocol     *rolldpos.Protocol
	// TODO: explorer dependency deleted at #1085, need to api related params
	api          *api.Server
	indexBuilder *blockdao.IndexBuilder
	registry     *protocol.Registry
}

type optionParams struct {
	isTesting  bool
	isSubchain bool
}

// Option sets ChainService construction parameter.
type Option func(ops *optionParams) error

// WithTesting is an option to create a testing ChainService.
func WithTesting() Option {
	return func(ops *optionParams) error {
		ops.isTesting = true
		return nil
	}
}

//WithSubChain is an option to create subChainService
func WithSubChain() Option {
	return func(ops *optionParams) error {
		ops.isSubchain = true
		return nil
	}
}

// New creates a ChainService from config and network.Overlay and dispatcher.Dispatcher.
func New(
	cfg config.Config,
	p2pAgent *p2p.Agent,
	dispatcher dispatcher.Dispatcher,
	opts ...Option,
) (*ChainService, error) {
	var err error
	var ops optionParams
	for _, opt := range opts {
		if err = opt(&ops); err != nil {
			return nil, err
		}
	}

	var chainOpts []blockchain.Option
	if ops.isTesting {
		chainOpts = []blockchain.Option{
			blockchain.InMemStateFactoryOption(),
		}
	} else {
		chainOpts = []blockchain.Option{
			blockchain.DefaultStateFactoryOption(),
		}
	}
	registry := protocol.Registry{}
	chainOpts = append(chainOpts, blockchain.RegistryOption(&registry))
	var electionCommittee committee.Committee
	if cfg.Genesis.EnableGravityChainVoting {
		committeeConfig := cfg.Chain.Committee
		committeeConfig.GravityChainStartHeight = cfg.Genesis.GravityChainStartHeight
		committeeConfig.GravityChainHeightInterval = cfg.Genesis.GravityChainHeightInterval
		committeeConfig.RegisterContractAddress = cfg.Genesis.RegisterContractAddress
		committeeConfig.StakingContractAddress = cfg.Genesis.StakingContractAddress
		committeeConfig.VoteThreshold = cfg.Genesis.VoteThreshold
		committeeConfig.ScoreThreshold = "0"
		committeeConfig.StakingContractAddress = cfg.Genesis.StakingContractAddress
		committeeConfig.SelfStakingThreshold = cfg.Genesis.SelfStakingThreshold

		if committeeConfig.GravityChainStartHeight != 0 {
			arch, err := committee.NewArchive(
				cfg.Chain.GravityChainDB.DbPath,
				cfg.Chain.GravityChainDB.NumRetries,
				committeeConfig.GravityChainStartHeight,
				committeeConfig.GravityChainHeightInterval,
			)
			if err != nil {
				return nil, err
			}
			if electionCommittee, err = committee.NewCommittee(arch, committeeConfig); err != nil {
				return nil, err
			}
		}
	}
	// create indexer
	var indexer blockindex.Indexer
	_, gateway := cfg.Plugins[config.GatewayPlugin]
	if gateway {
		var err error
		cfg.DB.DbPath = cfg.Chain.IndexDBPath
		indexer, err = blockindex.NewIndexer(db.NewBoltDB(cfg.DB), cfg.Genesis.Hash())
		if err != nil {
			return nil, err
		}
	}
	// create BlockDAO
	var kvstore db.KVStore
	if ops.isTesting {
		kvstore = db.NewMemKVStore()
	} else {
		cfg.DB.DbPath = cfg.Chain.ChainDBPath
		kvstore = db.NewBoltDB(cfg.DB)
	}
	var dao blockdao.BlockDAO
	if gateway && !cfg.Chain.EnableAsyncIndexWrite {
		dao = blockdao.NewBlockDAO(kvstore, indexer, cfg.Chain.CompressBlock, cfg.DB)
	} else {
		dao = blockdao.NewBlockDAO(kvstore, nil, cfg.Chain.CompressBlock, cfg.DB)
	}
	// create Blockchain
	chain := blockchain.NewBlockchain(cfg, dao, chainOpts...)
	if chain == nil {
		panic("failed to create blockchain")
	}
	// config asks for a standalone indexer
	var indexBuilder *blockdao.IndexBuilder
	if gateway && cfg.Chain.EnableAsyncIndexWrite {
		if indexBuilder, err = blockdao.NewIndexBuilder(chain.ChainID(), dao, indexer); err != nil {
			return nil, errors.Wrap(err, "failed to create index builder")
		}
		if err := chain.AddSubscriber(indexBuilder); err != nil {
			log.L().Warn("Failed to add subscriber: index builder.", zap.Error(err))
		}
	}
	// Create ActPool
	actOpts := make([]actpool.Option, 0)
	actPool, err := actpool.NewActPool(chain, cfg.ActPool, actOpts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create actpool")
	}
	rDPoSProtocol := rolldpos.NewProtocol(
		cfg.Genesis.NumCandidateDelegates,
		cfg.Genesis.NumDelegates,
		cfg.Genesis.NumSubEpochs,
		rolldpos.EnableDardanellesSubEpoch(cfg.Genesis.DardanellesBlockHeight, cfg.Genesis.DardanellesNumSubEpochs),
	)
	copts := []consensus.Option{
		consensus.WithBroadcast(func(msg proto.Message) error {
			return p2pAgent.BroadcastOutbound(p2p.WitContext(context.Background(), p2p.Context{ChainID: chain.ChainID()}), msg)
		}),
		consensus.WithRollDPoSProtocol(rDPoSProtocol),
	}
	// TODO: explorer dependency deleted at #1085, need to revive by migrating to api
	consensus, err := consensus.NewConsensus(cfg, chain, actPool, copts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create consensus")
	}
	bs, err := blocksync.NewBlockSyncer(
		cfg,
		chain,
		actPool,
		consensus,
		blocksync.WithUnicastOutBound(func(ctx context.Context, peer peerstore.PeerInfo, msg proto.Message) error {
			ctx = p2p.WitContext(ctx, p2p.Context{ChainID: chain.ChainID()})
			return p2pAgent.UnicastOutbound(ctx, peer, msg)
		}),
		blocksync.WithNeighbors(p2pAgent.Neighbors),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create blockSyncer")
	}

	var apiSvr *api.Server
	apiSvr, err = api.NewServer(
		cfg,
		chain,
		dao,
		indexer,
		actPool,
		&registry,
		api.WithBroadcastOutbound(func(ctx context.Context, chainID uint32, msg proto.Message) error {
			ctx = p2p.WitContext(ctx, p2p.Context{ChainID: chainID})
			return p2pAgent.BroadcastOutbound(ctx, msg)
		}),
		api.WithNativeElection(electionCommittee),
	)
	if err != nil {
		return nil, err
	}
	// Add action validators
	actPool.
		AddActionEnvelopeValidators(
			protocol.NewGenericValidator(chain.Factory().Nonce),
		)
	chain.Validator().
		AddActionEnvelopeValidators(
			protocol.NewGenericValidator(chain.Factory().Nonce),
		)
	if !ops.isSubchain {
		chain.Validator().
			SetActPool(
				actPool,
			)
	}

	cs := &ChainService{
		actpool:           actPool,
		chain:             chain,
		blocksync:         bs,
		consensus:         consensus,
		rDPoSProtocol:     rDPoSProtocol,
		electionCommittee: electionCommittee,
		indexBuilder:      indexBuilder,
		api:               apiSvr,
		registry:          &registry,
	}
	// Install protocols
	if err := cs.registerDefaultProtocols(cfg); err != nil {
		return nil, err
	}
	return cs, nil
}

// Start starts the server
func (cs *ChainService) Start(ctx context.Context) error {
	if cs.electionCommittee != nil {
		if err := cs.electionCommittee.Start(ctx); err != nil {
			return errors.Wrap(err, "error when starting election committee")
		}
	}
	if err := cs.chain.Start(ctx); err != nil {
		return errors.Wrap(err, "error when starting blockchain")
	}
	if err := cs.consensus.Start(ctx); err != nil {
		return errors.Wrap(err, "error when starting consensus")
	}
	if cs.indexBuilder != nil {
		if err := cs.indexBuilder.Start(ctx); err != nil {
			return errors.Wrap(err, "error when starting index builder")
		}
	}
	if err := cs.blocksync.Start(ctx); err != nil {
		return errors.Wrap(err, "error when starting blocksync")
	}
	// TODO: explorer dependency deleted at #1085, need to revive by migrating to api
	if cs.api != nil {
		if err := cs.api.Start(); err != nil {
			return errors.Wrap(err, "err when starting API server")
		}
	}

	return nil
}

// Stop stops the server
func (cs *ChainService) Stop(ctx context.Context) error {
	if cs.indexBuilder != nil {
		if err := cs.indexBuilder.Stop(ctx); err != nil {
			return errors.Wrap(err, "error when stopping index builder")
		}
	}
	// TODO: explorer dependency deleted at #1085, need to revive by migrating to api
	if cs.api != nil {
		if err := cs.api.Stop(); err != nil {
			return errors.Wrap(err, "error when stopping API server")
		}
	}
	if err := cs.consensus.Stop(ctx); err != nil {
		return errors.Wrap(err, "error when stopping consensus")
	}
	if err := cs.blocksync.Stop(ctx); err != nil {
		return errors.Wrap(err, "error when stopping blocksync")
	}
	if err := cs.chain.Stop(ctx); err != nil {
		return errors.Wrap(err, "error when stopping blockchain")
	}
	return nil
}

// HandleAction handles incoming action request.
func (cs *ChainService) HandleAction(_ context.Context, actPb *iotextypes.Action) error {
	var act action.SealedEnvelope
	if err := act.LoadProto(actPb); err != nil {
		return err
	}
	err := cs.actpool.Add(act)
	if err != nil {
		log.L().Debug(err.Error())
	}
	return err
}

// HandleBlock handles incoming block request.
func (cs *ChainService) HandleBlock(ctx context.Context, pbBlock *iotextypes.Block) error {
	blk := &block.Block{}
	if err := blk.ConvertFromBlockPb(pbBlock); err != nil {
		return err
	}
	return cs.blocksync.ProcessBlock(ctx, blk)
}

// HandleBlockSync handles incoming block sync request.
func (cs *ChainService) HandleBlockSync(ctx context.Context, pbBlock *iotextypes.Block) error {
	blk := &block.Block{}
	if err := blk.ConvertFromBlockPb(pbBlock); err != nil {
		return err
	}
	return cs.blocksync.ProcessBlockSync(ctx, blk)
}

// HandleSyncRequest handles incoming sync request.
func (cs *ChainService) HandleSyncRequest(ctx context.Context, peer peerstore.PeerInfo, sync *iotexrpc.BlockSync) error {
	return cs.blocksync.ProcessSyncRequest(ctx, peer, sync)
}

// HandleConsensusMsg handles incoming consensus message.
func (cs *ChainService) HandleConsensusMsg(msg *iotextypes.ConsensusMessage) error {
	return cs.consensus.HandleConsensusMsg(msg)
}

// ChainID returns ChainID.
func (cs *ChainService) ChainID() uint32 { return cs.chain.ChainID() }

// Blockchain returns the Blockchain
func (cs *ChainService) Blockchain() blockchain.Blockchain {
	return cs.chain
}

// ActionPool returns the Action pool
func (cs *ChainService) ActionPool() actpool.ActPool {
	return cs.actpool
}

// APIServer returns the API server
func (cs *ChainService) APIServer() *api.Server {
	return cs.api
}

// Consensus returns the consensus instance
func (cs *ChainService) Consensus() consensus.Consensus {
	return cs.consensus
}

// BlockSync returns the block syncer
func (cs *ChainService) BlockSync() blocksync.BlockSync {
	return cs.blocksync
}

// registerProtocol register a protocol
func (cs *ChainService) registerProtocol(id string, p protocol.Protocol) error {
	if err := cs.registry.Register(id, p); err != nil {
		return err
	}

	cs.actpool.AddActionValidators(p)
	return nil
}

// Registry returns a pointer to the registry
func (cs *ChainService) Registry() *protocol.Registry { return cs.registry }

// registerDefaultProtocols registers default protocol into chainservice's registry
func (cs *ChainService) registerDefaultProtocols(cfg config.Config) (err error) {
	accountProtocol := account.NewProtocol()
	if err = cs.registerProtocol(account.ProtocolID, accountProtocol); err != nil {
		return
	}
	if err = cs.registerProtocol(rolldpos.ProtocolID, cs.rDPoSProtocol); err != nil {
		return
	}
	pollProtocol, err := poll.NewProtocol(
		cfg,
		func(contract string, height uint64, ts time.Time, params []byte) ([]byte, error) {
			ex, err := action.NewExecution(contract, 1, big.NewInt(0), 1000000, big.NewInt(0), params)
			if err != nil {
				return nil, err
			}

			addr, err := address.FromString(address.ZeroAddress)
			if err != nil {
				return nil, err
			}

			data, _, err := blockchain.SimulateExecution(cs.chain, addr, ex)

			return data, err
		},
		cs.chain.CandidatesByHeight,
		cs.electionCommittee,
		func(height uint64) (time.Time, error) {
			header, err := cs.chain.BlockHeaderByHeight(height)
			if err != nil {
				return time.Now(), errors.Wrapf(
					err, "error when getting the block at height: %d",
					height,
				)
			}
			return header.Timestamp(), nil
		},
		cs.rDPoSProtocol,
	)
	if err != nil {
		return
	}
	if pollProtocol != nil {
		if err = cs.registerProtocol(poll.ProtocolID, pollProtocol); err != nil {
			return
		}
	}
	executionProtocol := execution.NewProtocol(cs.chain.BlockDAO().GetBlockHash)
	if err = cs.registerProtocol(execution.ProtocolID, executionProtocol); err != nil {
		return
	}
	rewardingProtocol := rewarding.NewProtocol(cs.chain, cs.rDPoSProtocol)
	return cs.registerProtocol(rewarding.ProtocolID, rewardingProtocol)
}
