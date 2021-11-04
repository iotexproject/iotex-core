// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package chainservice

import (
	"context"
	"math/big"
	"math/rand"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-election/committee"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
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
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state/factory"
)

// ChainService is a blockchain service with all blockchain components.
type ChainService struct {
	actpool           actpool.ActPool
	blocksync         blocksync.BlockSync
	consensus         consensus.Consensus
	chain             blockchain.Blockchain
	factory           factory.Factory
	blockdao          blockdao.BlockDAO
	p2pAgent          p2p.Agent
	electionCommittee committee.Committee
	// TODO: explorer dependency deleted at #1085, need to api related params
	api                *api.ServerV2
	indexBuilder       *blockindex.IndexBuilder
	bfIndexer          blockindex.BloomFilterIndexer
	candidateIndexer   *poll.CandidateIndexer
	candBucketsIndexer *staking.CandidatesBucketsIndexer
	registry           *protocol.Registry
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

// New creates a ChainService from config and network.Overlay
func New(
	cfg config.Config,
	p2pAgent p2p.Agent,
	opts ...Option,
) (*ChainService, error) {
	// create indexers
	var (
		indexers           []blockdao.BlockIndexer
		indexer            blockindex.Indexer
		bfIndexer          blockindex.BloomFilterIndexer
		candidateIndexer   *poll.CandidateIndexer
		candBucketsIndexer *staking.CandidatesBucketsIndexer
		err                error
		ops                optionParams
	)
	for _, opt := range opts {
		if err = opt(&ops); err != nil {
			return nil, err
		}
	}
	registry := protocol.NewRegistry()
	// create state factory
	var sf factory.Factory
	if ops.isTesting {
		sf, err = factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry))
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to create state factory")
		}
	} else {
		if cfg.Chain.EnableTrielessStateDB {
			opts := []factory.StateDBOption{
				factory.RegistryStateDBOption(registry),
				factory.DefaultPatchOption(),
			}
			if cfg.Chain.EnableStateDBCaching {
				opts = append(opts, factory.CachedStateDBOption())
			} else {
				opts = append(opts, factory.DefaultStateDBOption())
			}
			sf, err = factory.NewStateDB(cfg, opts...)
		} else {
			sf, err = factory.NewFactory(cfg, factory.DefaultTrieOption(), factory.RegistryOption(registry))
		}
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to create state factory")
		}
	}
	indexers = append(indexers, sf)
	var chainOpts []blockchain.Option
	var electionCommittee committee.Committee
	if cfg.Genesis.EnableGravityChainVoting {
		committeeConfig := cfg.Chain.Committee
		committeeConfig.GravityChainStartHeight = cfg.Genesis.GravityChainStartHeight
		committeeConfig.GravityChainCeilingHeight = cfg.Genesis.GravityChainCeilingHeight
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
	_, gateway := cfg.Plugins[config.GatewayPlugin]
	if gateway {
		cfg.DB.DbPath = cfg.Chain.IndexDBPath
		indexer, err = blockindex.NewIndexer(db.NewBoltDB(cfg.DB), cfg.Genesis.Hash())
		if err != nil {
			return nil, err
		}
		if !cfg.Chain.EnableAsyncIndexWrite {
			indexers = append(indexers, indexer)
		}

		// create bloomfilter indexer
		cfg.DB.DbPath = cfg.Chain.BloomfilterIndexDBPath
		bfIndexer, err = blockindex.NewBloomfilterIndexer(db.NewBoltDB(cfg.DB), cfg.Indexer)
		if err != nil {
			return nil, err
		}
		indexers = append(indexers, bfIndexer)

		// create candidate indexer
		cfg.DB.DbPath = cfg.Chain.CandidateIndexDBPath
		candidateIndexer, err = poll.NewCandidateIndexer(db.NewBoltDB(cfg.DB))
		if err != nil {
			return nil, err
		}
		if cfg.Chain.EnableStakingIndexer {
			cfg.DB.DbPath = cfg.Chain.StakingIndexDBPath
			candBucketsIndexer, err = staking.NewStakingCandidatesBucketsIndexer(db.NewBoltDB(cfg.DB))
			if err != nil {
				return nil, err
			}
		}
	}

	// create BlockDAO
	var dao blockdao.BlockDAO
	if ops.isTesting {
		dao = blockdao.NewBlockDAOInMemForTest(indexers)
	} else {
		cfg.DB.DbPath = cfg.Chain.ChainDBPath
		cfg.DB.CompressLegacy = cfg.Chain.CompressBlock
		dao = blockdao.NewBlockDAO(indexers, cfg.DB)
	}

	// Create ActPool
	actOpts := make([]actpool.Option, 0)
	actPool, err := actpool.NewActPool(sf, cfg.ActPool, actOpts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create actpool")
	}

	// Add action validators
	actPool.AddActionEnvelopeValidators(
		protocol.NewGenericValidator(sf, accountutil.AccountState),
	)
	if !ops.isSubchain {
		chainOpts = append(chainOpts, blockchain.BlockValidatorOption(block.NewValidator(sf, actPool)))
	} else {
		chainOpts = append(chainOpts, blockchain.BlockValidatorOption(sf))
	}

	// create Blockchain
	chain := blockchain.NewBlockchain(cfg, dao, factory.NewMinter(sf, actPool), chainOpts...)
	if chain == nil {
		panic("failed to create blockchain")
	}
	if err := chain.AddSubscriber(actPool); err != nil {
		return nil, errors.Wrap(err, "failed to add subscriber: action pool.")
	}
	// config asks for a standalone indexer
	var indexBuilder *blockindex.IndexBuilder
	if gateway && cfg.Chain.EnableAsyncIndexWrite {
		if indexBuilder, err = blockindex.NewIndexBuilder(chain.ChainID(), dao, indexer); err != nil {
			return nil, errors.Wrap(err, "failed to create index builder")
		}
		if err := chain.AddSubscriber(indexBuilder); err != nil {
			log.L().Warn("Failed to add subscriber: index builder.", zap.Error(err))
		}
	}
	copts := []consensus.Option{
		consensus.WithBroadcast(func(msg proto.Message) error {
			return p2pAgent.BroadcastOutbound(context.Background(), msg)
		}),
	}
	var (
		rDPoSProtocol   *rolldpos.Protocol
		pollProtocol    poll.Protocol
		stakingProtocol *staking.Protocol
	)
	// staking protocol need to be put in registry before poll protocol when enabling
	if cfg.Chain.EnableStakingProtocol {
		stakingProtocol, err = staking.NewProtocol(
			rewarding.DepositGas,
			cfg.Genesis.Staking,
			candBucketsIndexer,
			cfg.Genesis.GreenlandBlockHeight,
			cfg.Genesis.HawaiiBlockHeight,
		)
		if err != nil {
			return nil, err
		}
		if err = stakingProtocol.Register(registry); err != nil {
			return nil, err
		}
	}
	if cfg.Consensus.Scheme == config.RollDPoSScheme {
		rDPoSProtocol = rolldpos.NewProtocol(
			cfg.Genesis.NumCandidateDelegates,
			cfg.Genesis.NumDelegates,
			cfg.Genesis.NumSubEpochs,
			rolldpos.EnableDardanellesSubEpoch(cfg.Genesis.DardanellesBlockHeight, cfg.Genesis.DardanellesNumSubEpochs),
		)
		copts = append(copts, consensus.WithRollDPoSProtocol(rDPoSProtocol))
		pollProtocol, err = poll.NewProtocol(
			cfg,
			candidateIndexer,
			func(ctx context.Context, contract string, params []byte, correctGas bool) ([]byte, error) {
				gasLimit := uint64(1000000)
				if correctGas {
					gasLimit *= 10
				}
				ex, err := action.NewExecution(contract, 1, big.NewInt(0), gasLimit, big.NewInt(0), params)
				if err != nil {
					return nil, err
				}

				addr, err := address.FromString(address.ZeroAddress)
				if err != nil {
					return nil, err
				}

				data, _, err := sf.SimulateExecution(ctx, addr, ex, dao.GetBlockHash)

				return data, err
			},
			candidatesutil.CandidatesFromDB,
			candidatesutil.ProbationListFromDB,
			candidatesutil.UnproductiveDelegateFromDB,
			electionCommittee,
			stakingProtocol,
			func(height uint64) (time.Time, error) {
				header, err := chain.BlockHeaderByHeight(height)
				if err != nil {
					return time.Now(), errors.Wrapf(
						err, "error when getting the block at height: %d",
						height,
					)
				}
				return header.Timestamp(), nil
			},
			func(start, end uint64) (map[string]uint64, error) {
				return blockchain.Productivity(chain, start, end)
			},
			dao.GetBlockHash,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to generate poll protocol")
		}
		if pollProtocol != nil {
			copts = append(copts, consensus.WithPollProtocol(pollProtocol))
		}
	}
	// TODO: rewarding protocol for standalone mode is weird, rDPoSProtocol could be passed via context
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	// TODO: explorer dependency deleted at #1085, need to revive by migrating to api
	consensus, err := consensus.NewConsensus(cfg, chain, sf, copts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create consensus")
	}
	var bs blocksync.BlockSync
	switch cfg.Consensus.Scheme {
	case config.StandaloneScheme:
		bs = blocksync.NewDummyBlockSyncer()
	default:
		bs, err = blocksync.NewBlockSyncer(
			cfg.BlockSync,
			chain.TipHeight,
			func(height uint64) (*block.Block, error) {
				return dao.GetBlockByHeight(height)
			},
			func(blk *block.Block) error {
				if err := consensus.ValidateBlockFooter(blk); err != nil {
					log.L().Debug("Failed to validate block footer.", zap.Error(err), zap.Uint64("height", blk.Height()))
					return err
				}
				retries := 1
				if !cfg.Genesis.IsHawaii(blk.Height()) {
					retries = 4
				}
				var err error
				for i := 0; i < retries; i++ {
					if err = chain.ValidateBlock(blk); err == nil {
						if err = chain.CommitBlock(blk); err == nil {
							break
						}
					}
					switch errors.Cause(err) {
					case blockchain.ErrInvalidTipHeight:
						log.L().Debug("Skip block.", zap.Error(err), zap.Uint64("height", blk.Height()))
						return nil
					case block.ErrDeltaStateMismatch:
						log.L().Debug("Delta state mismatched.", zap.Uint64("height", blk.Height()))
					default:
						log.L().Debug("Failed to commit the block.", zap.Error(err), zap.Uint64("height", blk.Height()))
						return err
					}
				}
				if err != nil {
					log.L().Debug("Failed to commit block.", zap.Error(err), zap.Uint64("height", blk.Height()))
					return err
				}
				log.L().Info("Successfully committed block.", zap.Uint64("height", blk.Height()))
				consensus.Calibrate(blk.Height())

				return nil
			},
			func(ctx context.Context, start uint64, end uint64, repeat int) {
				peers, err := p2pAgent.Neighbors(ctx)
				if err != nil {
					log.L().Error("failed to get neighbours", zap.Error(err))
					return
				}
				if len(peers) == 0 {
					log.L().Error("no peers")
				}
				if repeat < 2 {
					repeat = 2
				}
				if repeat > len(peers) {
					repeat = len(peers)
				}
				for i := 0; i < repeat; i++ {
					peer := peers[rand.Intn(len(peers)-i)]
					if err := p2pAgent.UnicastOutbound(
						ctx,
						peer,
						&iotexrpc.BlockSync{Start: start, End: end},
					); err != nil {
						log.L().Error("failed to request blocks", zap.Error(err), zap.String("peer", peer.ID.Pretty()), zap.Uint64("start", start), zap.Uint64("end", end))
					}
				}
			},
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create blockSyncer")
		}
	}

	apiSvr, err := api.NewServerV2(
		cfg,
		chain,
		bs,
		sf,
		dao,
		indexer,
		bfIndexer,
		actPool,
		registry,
		api.WithBroadcastOutbound(func(ctx context.Context, chainID uint32, msg proto.Message) error {
			return p2pAgent.BroadcastOutbound(ctx, msg)
		}),
		api.WithNativeElection(electionCommittee),
	)
	if err != nil {
		return nil, err
	}
	accountProtocol := account.NewProtocol(rewarding.DepositGas)
	if accountProtocol != nil {
		if err = accountProtocol.Register(registry); err != nil {
			return nil, err
		}
	}
	if rDPoSProtocol != nil {
		if err = rDPoSProtocol.Register(registry); err != nil {
			return nil, err
		}
	}
	if pollProtocol != nil {
		if err = pollProtocol.Register(registry); err != nil {
			return nil, err
		}
	}
	executionProtocol := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas)
	if executionProtocol != nil {
		if err = executionProtocol.Register(registry); err != nil {
			return nil, err
		}
	}
	if rewardingProtocol != nil {
		if err = rewardingProtocol.Register(registry); err != nil {
			return nil, err
		}
	}

	return &ChainService{
		actpool:            actPool,
		chain:              chain,
		factory:            sf,
		blockdao:           dao,
		blocksync:          bs,
		p2pAgent:           p2pAgent,
		consensus:          consensus,
		electionCommittee:  electionCommittee,
		indexBuilder:       indexBuilder,
		bfIndexer:          bfIndexer,
		candidateIndexer:   candidateIndexer,
		candBucketsIndexer: candBucketsIndexer,
		api:                apiSvr,
		registry:           registry,
	}, nil
}

// Start starts the server
func (cs *ChainService) Start(ctx context.Context) error {
	if cs.electionCommittee != nil {
		if err := cs.electionCommittee.Start(ctx); err != nil {
			return errors.Wrap(err, "error when starting election committee")
		}
	}
	if cs.candidateIndexer != nil {
		if err := cs.candidateIndexer.Start(ctx); err != nil {
			return errors.Wrap(err, "error when starting candidate indexer")
		}
	}
	if cs.candBucketsIndexer != nil {
		if err := cs.candBucketsIndexer.Start(ctx); err != nil {
			return errors.Wrap(err, "error when starting staking candidates indexer")
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
	if cs.candidateIndexer != nil {
		if err := cs.candidateIndexer.Stop(ctx); err != nil {
			return errors.Wrap(err, "error when stopping candidate indexer")
		}
	}
	if cs.candBucketsIndexer != nil {
		if err := cs.candBucketsIndexer.Stop(ctx); err != nil {
			return errors.Wrap(err, "error when stopping staking candidates indexer")
		}
	}
	if cs.electionCommittee != nil {
		return cs.electionCommittee.Stop(ctx)
	}
	return nil
}

// ReportFullness switch on or off block sync
func (cs *ChainService) ReportFullness(_ context.Context, _ iotexrpc.MessageType, fullness float32) {
}

// HandleAction handles incoming action request.
func (cs *ChainService) HandleAction(ctx context.Context, actPb *iotextypes.Action) error {
	var act action.SealedEnvelope
	if err := act.LoadProto(actPb); err != nil {
		return err
	}
	ctx = protocol.WithRegistry(ctx, cs.registry)
	err := cs.actpool.Add(ctx, act)
	if err != nil {
		log.L().Debug(err.Error())
	}
	return err
}

// HandleBlock handles incoming block request.
func (cs *ChainService) HandleBlock(ctx context.Context, peer string, pbBlock *iotextypes.Block) error {
	blk := &block.Block{}
	if err := blk.ConvertFromBlockPb(pbBlock); err != nil {
		return err
	}
	ctx, err := cs.chain.Context(ctx)
	if err != nil {
		return err
	}
	return cs.blocksync.ProcessBlock(ctx, peer, blk)
}

// HandleSyncRequest handles incoming sync request.
func (cs *ChainService) HandleSyncRequest(ctx context.Context, peer peer.AddrInfo, sync *iotexrpc.BlockSync) error {
	return cs.blocksync.ProcessSyncRequest(ctx, sync.Start, sync.End, func(ctx context.Context, blk *block.Block) error {
		return cs.p2pAgent.UnicastOutbound(
			ctx,
			peer,
			blk.ConvertToBlockPb(),
		)
	})
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

// StateFactory returns the state factory
func (cs *ChainService) StateFactory() factory.Factory {
	return cs.factory
}

// BlockDAO returns the blockdao
func (cs *ChainService) BlockDAO() blockdao.BlockDAO {
	return cs.blockdao
}

// ActionPool returns the Action pool
func (cs *ChainService) ActionPool() actpool.ActPool {
	return cs.actpool
}

// APIServer returns the API server
func (cs *ChainService) APIServer() *api.CoreService {
	return cs.api.CoreService()
}

// Consensus returns the consensus instance
func (cs *ChainService) Consensus() consensus.Consensus {
	return cs.consensus
}

// BlockSync returns the block syncer
func (cs *ChainService) BlockSync() blocksync.BlockSync {
	return cs.blocksync
}

// Registry returns a pointer to the registry
func (cs *ChainService) Registry() *protocol.Registry { return cs.registry }
