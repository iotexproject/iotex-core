// Copyright (c) 2022 IoTeX Foundation
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

	"github.com/iotexproject/iotex-address/address"
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
	"github.com/iotexproject/iotex-core/gasstation"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-election/committee"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// Builder is a builder to build chainservice
type Builder struct {
	cfg config.Config
	cs  *ChainService
}

// NewBuilder creates a new chainservice builder
func NewBuilder(cfg config.Config) *Builder {
	builder := &Builder{cfg: cfg}
	builder.createInstance()

	return builder
}

// SetActionPool sets the action pool instance
func (builder *Builder) SetActionPool(ap actpool.ActPool) *Builder {
	builder.createInstance()
	builder.cs.actpool = ap
	return builder
}

// SetBlockchain sets the blockchain instance
func (builder *Builder) SetBlockchain(bc blockchain.Blockchain) *Builder {
	builder.createInstance()
	builder.cs.chain = bc
	return builder
}

// SetFactory sets the factory instance
func (builder *Builder) SetFactory(f factory.Factory) *Builder {
	builder.createInstance()
	builder.cs.factory = f
	return builder
}

// SetBlockDAO sets the blockdao instance
func (builder *Builder) SetBlockDAO(bd blockdao.BlockDAO) *Builder {
	builder.createInstance()
	builder.cs.blockdao = bd
	return builder
}

// SetP2PAgent sets the P2PAgent instance
func (builder *Builder) SetP2PAgent(agent p2p.Agent) *Builder {
	builder.createInstance()
	builder.cs.p2pAgent = agent
	return builder
}

// SetElectionCommittee sets the election committee instance
func (builder *Builder) SetElectionCommittee(c committee.Committee) *Builder {
	builder.createInstance()
	builder.cs.electionCommittee = c
	return builder
}

// SetBlockSync sets the block sync instance
func (builder *Builder) SetBlockSync(bs blocksync.BlockSync) *Builder {
	builder.createInstance()
	builder.cs.blocksync = bs
	return builder
}

// SetRegistry sets the registry instance
func (builder *Builder) SetRegistry(registry *protocol.Registry) *Builder {
	builder.createInstance()
	builder.cs.registry = registry
	return builder
}

// SetBloomFilterIndexer sets the bloom filter indexer
func (builder *Builder) SetBloomFilterIndexer(indexer blockindex.BloomFilterIndexer) *Builder {
	builder.createInstance()
	builder.cs.bfIndexer = indexer
	return builder
}

// BuildForTest builds a chainservice for test purpose
func (builder *Builder) BuildForTest() (*ChainService, error) {
	builder.createInstance()
	return builder.build(false, true)
}

// BuildForSubChain builds a chainservice for subchain
func (builder *Builder) BuildForSubChain() (*ChainService, error) {
	builder.createInstance()
	return builder.build(true, false)
}

// Build builds a chainservice
func (builder *Builder) Build() (*ChainService, error) {
	builder.createInstance()
	return builder.build(false, false)
}

func (builder *Builder) createInstance() {
	if builder.cs == nil {
		builder.cs = &ChainService{
			readCache: NewReadCache(),
		}
	}
}

func (builder *Builder) buildFactory(forTest bool) error {
	factory, err := builder.createFactory(forTest)
	if err != nil {
		return errors.Wrapf(err, "failed to create state factory")
	}
	builder.cs.factory = factory
	return nil
}

func (builder *Builder) createFactory(forTest bool) (factory.Factory, error) {
	if builder.cs.factory != nil {
		return builder.cs.factory, nil
	}
	if forTest {
		return factory.NewFactory(builder.cfg, factory.InMemTrieOption(), factory.RegistryOption(builder.cs.registry))
	}

	if builder.cfg.Chain.EnableTrielessStateDB {
		if forTest {
			return factory.NewStateDB(builder.cfg, factory.InMemStateDBOption(), factory.RegistryStateDBOption(builder.cs.registry))
		}
		opts := []factory.StateDBOption{
			factory.RegistryStateDBOption(builder.cs.registry),
			factory.DefaultPatchOption(),
		}
		if builder.cfg.Chain.EnableStateDBCaching {
			opts = append(opts, factory.CachedStateDBOption())
		} else {
			opts = append(opts, factory.DefaultStateDBOption())
		}
		return factory.NewStateDB(builder.cfg, opts...)
	}
	if forTest {
		return factory.NewFactory(builder.cfg, factory.InMemTrieOption(), factory.RegistryOption(builder.cs.registry))
	}

	return factory.NewFactory(
		builder.cfg,
		factory.DefaultTrieOption(),
		factory.RegistryOption(builder.cs.registry),
		factory.DefaultTriePatchOption(),
	)
}

func (builder *Builder) buildElectionCommittee() error {
	ec, err := builder.createElectionCommittee()
	if err != nil {
		return errors.Wrapf(err, "failed to create election committee")
	}
	if ec != nil {
		builder.cs.electionCommittee = ec
		builder.cs.lifecycle.Add(ec)
	}
	return nil
}

func (builder *Builder) createElectionCommittee() (committee.Committee, error) {
	if builder.cs.electionCommittee != nil {
		return builder.cs.electionCommittee, nil
	}
	if !builder.cfg.Genesis.EnableGravityChainVoting {
		return nil, nil
	}
	cfg := builder.cfg

	committeeConfig := builder.cfg.Chain.Committee
	if committeeConfig.GravityChainStartHeight == 0 {
		return nil, nil
	}
	committeeConfig.GravityChainStartHeight = builder.cfg.Genesis.GravityChainStartHeight
	committeeConfig.GravityChainCeilingHeight = cfg.Genesis.GravityChainCeilingHeight
	committeeConfig.GravityChainHeightInterval = cfg.Genesis.GravityChainHeightInterval
	committeeConfig.RegisterContractAddress = cfg.Genesis.RegisterContractAddress
	committeeConfig.StakingContractAddress = cfg.Genesis.StakingContractAddress
	committeeConfig.VoteThreshold = cfg.Genesis.VoteThreshold
	committeeConfig.ScoreThreshold = "0"
	committeeConfig.StakingContractAddress = cfg.Genesis.StakingContractAddress
	committeeConfig.SelfStakingThreshold = cfg.Genesis.SelfStakingThreshold

	arch, err := committee.NewArchive(
		cfg.Chain.GravityChainDB.DbPath,
		cfg.Chain.GravityChainDB.NumRetries,
		committeeConfig.GravityChainStartHeight,
		committeeConfig.GravityChainHeightInterval,
	)
	if err != nil {
		return nil, err
	}
	return committee.NewCommittee(arch, committeeConfig)
}

func (builder *Builder) buildActionPool() error {
	if builder.cs.actpool == nil {
		ac, err := actpool.NewActPool(builder.cs.factory, builder.cfg.ActPool)
		if err != nil {
			return errors.Wrap(err, "failed to create actpool")
		}
		builder.cs.actpool = ac
	}
	// Add action validators
	builder.cs.actpool.AddActionEnvelopeValidators(
		protocol.NewGenericValidator(builder.cs.factory, accountutil.AccountState),
	)

	return nil
}

func (builder *Builder) buildBlockDAO(forTest bool) error {
	if builder.cs.blockdao != nil {
		return nil
	}

	var indexers []blockdao.BlockIndexer
	indexers = append(indexers, builder.cs.factory)
	if !builder.cfg.Chain.EnableAsyncIndexWrite {
		if builder.cs.bfIndexer != nil {
			indexers = append(indexers, builder.cs.bfIndexer)
		}
		if builder.cs.indexer != nil {
			indexers = append(indexers, builder.cs.indexer)
		}
	}
	if forTest {
		builder.cs.blockdao = blockdao.NewBlockDAOInMemForTest(indexers)
	} else {
		dbConfig := builder.cfg.DB
		dbConfig.DbPath = builder.cfg.Chain.ChainDBPath
		dbConfig.CompressLegacy = builder.cfg.Chain.CompressBlock

		builder.cs.blockdao = blockdao.NewBlockDAO(indexers, dbConfig)
	}

	return nil
}

func (builder *Builder) buildStakingIndexer(forTest bool) error {
	if !builder.cfg.Chain.EnableStakingIndexer {
		return nil
	}
	var store db.KVStoreForRangeIndex
	if forTest {
		store = db.NewMemKVStore()
	} else {
		dbConfig := builder.cfg.DB
		dbConfig.DbPath = builder.cfg.Chain.StakingIndexDBPath

		store = db.NewBoltDB(dbConfig)
	}
	indexer, err := staking.NewStakingCandidatesBucketsIndexer(store)
	if err != nil {
		return errors.Wrap(err, "failed to create staking candidate buckets indexer")
	}
	builder.cs.candBucketsIndexer = indexer
	builder.cs.lifecycle.Add(builder.cs.candBucketsIndexer)

	return nil
}

func (builder *Builder) buildGatewayComponents(forTest bool) error {
	indexer, bfIndexer, candidateIndexer, err := builder.createGateWayComponents(forTest)
	if err != nil {
		return errors.Wrapf(err, "failed to create gateway components")
	}
	builder.cs.candidateIndexer = candidateIndexer
	if builder.cs.candidateIndexer != nil {
		builder.cs.lifecycle.Add(builder.cs.candidateIndexer)
	}
	builder.cs.bfIndexer = bfIndexer
	builder.cs.indexer = indexer

	return nil
}

func (builder *Builder) createGateWayComponents(forTest bool) (
	indexer blockindex.Indexer,
	bfIndexer blockindex.BloomFilterIndexer,
	candidateIndexer *poll.CandidateIndexer,
	err error,
) {
	_, gateway := builder.cfg.Plugins[config.GatewayPlugin]
	if !gateway {
		return
	}
	bfIndexer = builder.cs.bfIndexer

	if forTest {
		indexer, err = blockindex.NewIndexer(db.NewMemKVStore(), builder.cfg.Genesis.Hash())
		if err != nil {
			return
		}
		if bfIndexer == nil {
			bfIndexer, err = blockindex.NewBloomfilterIndexer(db.NewMemKVStore(), builder.cfg.Indexer)
			if err != nil {
				return
			}
		}
		candidateIndexer, err = poll.NewCandidateIndexer(db.NewMemKVStore())

		return
	}
	dbConfig := builder.cfg.DB
	dbConfig.DbPath = builder.cfg.Chain.IndexDBPath
	indexer, err = blockindex.NewIndexer(db.NewBoltDB(dbConfig), builder.cfg.Genesis.Hash())
	if err != nil {
		return
	}

	if bfIndexer == nil {
		dbConfig.DbPath = builder.cfg.Chain.BloomfilterIndexDBPath
		bfIndexer, err = blockindex.NewBloomfilterIndexer(db.NewBoltDB(dbConfig), builder.cfg.Indexer)
		if err != nil {
			return
		}
	}

	// create candidate indexer
	dbConfig.DbPath = builder.cfg.Chain.CandidateIndexDBPath
	candidateIndexer, err = poll.NewCandidateIndexer(db.NewBoltDB(dbConfig))

	return
}

func (builder *Builder) buildBlockchain(forSubChain, forTest bool) error {
	builder.cs.chain = builder.createBlockchain(forSubChain, forTest)
	builder.cs.lifecycle.Add(builder.cs.chain)

	if err := builder.cs.chain.AddSubscriber(builder.cs.actpool); err != nil {
		return errors.Wrap(err, "failed to add actpool as subscriber")
	}
	if builder.cs.indexer != nil && builder.cfg.Chain.EnableAsyncIndexWrite {
		// config asks for a standalone indexer
		indexBuilder, err := blockindex.NewIndexBuilder(builder.cs.chain.ChainID(), builder.cs.blockdao, builder.cs.indexer)
		if err != nil {
			return errors.Wrap(err, "failed to create index builder")
		}
		builder.cs.lifecycle.Add(indexBuilder)
		if err := builder.cs.chain.AddSubscriber(indexBuilder); err != nil {
			return errors.Wrap(err, "failed to add index builder as subscriber")
		}
	}
	return nil
}

func (builder *Builder) createBlockchain(forSubChain, forTest bool) blockchain.Blockchain {
	if builder.cs.chain != nil {
		return builder.cs.chain
	}
	var chainOpts []blockchain.Option
	if !forSubChain {
		chainOpts = append(chainOpts, blockchain.BlockValidatorOption(block.NewValidator(builder.cs.factory, builder.cs.actpool)))
	} else {
		chainOpts = append(chainOpts, blockchain.BlockValidatorOption(builder.cs.factory))
	}

	return blockchain.NewBlockchain(builder.cfg, builder.cs.blockdao, factory.NewMinter(builder.cs.factory, builder.cs.actpool), chainOpts...)
}

func (builder *Builder) buildBlockSyncer() error {
	if builder.cs.blocksync != nil {
		return nil
	}
	if builder.cfg.Consensus.Scheme == config.StandaloneScheme {
		builder.cs.blocksync = blocksync.NewDummyBlockSyncer()
		return nil
	}

	p2pAgent := builder.cs.p2pAgent
	chain := builder.cs.chain
	consens := builder.cs.consensus

	blocksync, err := blocksync.NewBlockSyncer(
		builder.cfg.BlockSync,
		chain.TipHeight,
		builder.cs.blockdao.GetBlockByHeight,
		func(blk *block.Block) error {
			if err := consens.ValidateBlockFooter(blk); err != nil {
				log.L().Debug("Failed to validate block footer.", zap.Error(err), zap.Uint64("height", blk.Height()))
				return err
			}
			retries := 1
			if !builder.cfg.Genesis.IsHawaii(blk.Height()) {
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
			consens.Calibrate(blk.Height())
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
		return errors.Wrap(err, "failed to create block syncer")
	}
	builder.cs.blocksync = blocksync
	builder.cs.lifecycle.Add(blocksync)

	return nil
}

func (builder *Builder) registerStakingProtocol() error {
	if !builder.cfg.Chain.EnableStakingProtocol {
		return nil
	}
	if staking.FindProtocol(builder.cs.registry) != nil {
		return nil
	}
	stakingProtocol, err := staking.NewProtocol(
		rewarding.DepositGas,
		builder.cfg.Genesis.Staking,
		builder.cs.candBucketsIndexer,
		builder.cfg.Genesis.GreenlandBlockHeight,
		builder.cfg.Genesis.HawaiiBlockHeight,
	)
	if err != nil {
		return err
	}

	return stakingProtocol.Register(builder.cs.registry)
}

func (builder *Builder) registerRewardingProtocol() error {
	if rewarding.FindProtocol(builder.cs.registry) != nil {
		return nil
	}

	// TODO: rewarding protocol for standalone mode is weird, rDPoSProtocol could be passed via context
	return rewarding.NewProtocol(builder.cfg.Genesis.Rewarding).Register(builder.cs.registry)
}

func (builder *Builder) registerAccountProtocol() error {
	if account.FindProtocol(builder.cs.registry) != nil {
		return nil
	}

	return account.NewProtocol(rewarding.DepositGas).Register(builder.cs.registry)
}

func (builder *Builder) registerExecutionProtocol() error {
	if execution.FindProtocol(builder.cs.registry) != nil {
		return nil
	}

	return execution.NewProtocol(builder.cs.blockdao.GetBlockHash, rewarding.DepositGas).Register(builder.cs.registry)
}

func (builder *Builder) registerRollDPoSProtocol() error {
	if builder.cfg.Consensus.Scheme != config.RollDPoSScheme {
		return nil
	}
	if rolldpos.FindProtocol(builder.cs.registry) == nil {
		if err := rolldpos.NewProtocol(
			builder.cfg.Genesis.NumCandidateDelegates,
			builder.cfg.Genesis.NumDelegates,
			builder.cfg.Genesis.NumSubEpochs,
			rolldpos.EnableDardanellesSubEpoch(builder.cfg.Genesis.DardanellesBlockHeight, builder.cfg.Genesis.DardanellesNumSubEpochs),
		).Register(builder.cs.registry); err != nil {
			return err
		}
	}
	if poll.FindProtocol(builder.cs.registry) != nil {
		return nil
	}
	factory := builder.cs.factory
	dao := builder.cs.blockdao
	chain := builder.cs.chain
	pollProtocol, err := poll.NewProtocol(
		builder.cfg,
		builder.cs.candidateIndexer,
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

			data, _, err := factory.SimulateExecution(ctx, addr, ex, dao.GetBlockHash)

			return data, err
		},
		candidatesutil.CandidatesFromDB,
		candidatesutil.ProbationListFromDB,
		candidatesutil.UnproductiveDelegateFromDB,
		builder.cs.electionCommittee,
		staking.FindProtocol(builder.cs.registry),
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
		builder.cs.blockdao.GetBlockHash,
	)
	if err != nil {
		return errors.Wrap(err, "failed to generate poll protocol")
	}
	return pollProtocol.Register(builder.cs.registry)
}

func (builder *Builder) buildConsensusComponent() error {
	p2pAgent := builder.cs.p2pAgent
	copts := []consensus.Option{
		consensus.WithBroadcast(func(msg proto.Message) error {
			return p2pAgent.BroadcastOutbound(context.Background(), msg)
		}),
	}
	if rDPoSProtocol := rolldpos.FindProtocol(builder.cs.registry); rDPoSProtocol != nil {
		copts = append(copts, consensus.WithRollDPoSProtocol(rDPoSProtocol))
	}
	if pollProtocol := poll.FindProtocol(builder.cs.registry); pollProtocol != nil {
		copts = append(copts, consensus.WithPollProtocol(pollProtocol))
	}

	// TODO: explorer dependency deleted at #1085, need to revive by migrating to api
	component, err := consensus.NewConsensus(builder.cfg, builder.cs.chain, builder.cs.factory, copts...)
	if err != nil {
		return errors.Wrap(err, "failed to create consensus component")
	}
	builder.cs.consensus = component
	builder.cs.lifecycle.Add(component)

	return nil
}

func (builder *Builder) buildGasStation() error {
	builder.cs.gs = gasstation.NewGasStation(builder.cs.chain, builder.cs.factory.SimulateExecution, builder.cs.blockdao, builder.cfg.API.GasStation)
	return nil
}

func (builder *Builder) buildAPIServer() error {
	if builder.cfg.API.Port == 0 && builder.cfg.API.Web3Port == 0 {
		return nil
	}
	svr, err := api.NewServerV2(
		builder.cfg.API,
		builder.cs,
	)
	if err != nil {
		return errors.Wrap(err, "failed to create API server")
	}
	if err := builder.cs.chain.AddSubscriber(svr); err != nil {
		return errors.Wrap(err, "failed to add api server as subscriber")
	}
	builder.cs.api = svr

	return nil
}

func (builder *Builder) build(forSubChain, forTest bool) (*ChainService, error) {
	if builder.cs.registry == nil {
		builder.cs.registry = protocol.NewRegistry()
	}
	if builder.cs.p2pAgent == nil {
		builder.cs.p2pAgent = p2p.NewDummyAgent()
	}
	if err := builder.buildFactory(forTest); err != nil {
		return nil, err
	}
	if err := builder.buildElectionCommittee(); err != nil {
		return nil, err
	}
	if err := builder.buildActionPool(); err != nil {
		return nil, err
	}
	if err := builder.buildGatewayComponents(forTest); err != nil {
		return nil, err
	}
	if err := builder.buildBlockDAO(forTest); err != nil {
		return nil, err
	}
	if err := builder.buildStakingIndexer(forTest); err != nil {
		return nil, err
	}
	if err := builder.buildBlockchain(forSubChain, forTest); err != nil {
		return nil, err
	}
	if err := builder.buildGasStation(); err != nil {
		return nil, err
	}
	// staking protocol need to be put in registry before poll protocol when enabling
	if err := builder.registerStakingProtocol(); err != nil {
		return nil, errors.Wrap(err, "failed to register staking protocol")
	}
	if err := builder.registerRewardingProtocol(); err != nil {
		return nil, errors.Wrap(err, "failed to register rewarding protocol")
	}
	if err := builder.registerAccountProtocol(); err != nil {
		return nil, errors.Wrap(err, "failed to register rewarding protocol")
	}
	if err := builder.registerExecutionProtocol(); err != nil {
		return nil, errors.Wrap(err, "failed to register execution protocol")
	}
	if err := builder.registerRollDPoSProtocol(); err != nil {
		return nil, errors.Wrap(err, "failed to register roll dpos related protocols")
	}
	if err := builder.buildConsensusComponent(); err != nil {
		return nil, err
	}
	if err := builder.buildBlockSyncer(); err != nil {
		return nil, err
	}
	if err := builder.buildAPIServer(); err != nil {
		return nil, err
	}
	cs := builder.cs
	builder.cs = nil

	return cs, nil
}
