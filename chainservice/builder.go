// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package chainservice

import (
	"context"
	"math/big"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-election/committee"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/blockindex/contractstaking"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/consensus/consensusfsm"
	rp "github.com/iotexproject/iotex-core/consensus/scheme/rolldpos"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/nodeinfo"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/blockutil"
	"github.com/iotexproject/iotex-core/server/itx/nodestats"
	"github.com/iotexproject/iotex-core/state/factory"
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

// SetRPCStats sets the RPCStats instance
func (builder *Builder) SetRPCStats(stats *nodestats.APILocalStats) *Builder {
	builder.createInstance()
	builder.cs.apiStats = stats
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
		builder.cs = &ChainService{}
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
	var dao db.KVStore
	var err error
	if builder.cs.factory != nil {
		return builder.cs.factory, nil
	}
	factoryCfg := factory.GenerateConfig(builder.cfg.Chain, builder.cfg.Genesis)
	if builder.cfg.Chain.EnableTrielessStateDB {
		if forTest {
			return factory.NewStateDB(factoryCfg, db.NewMemKVStore(), factory.RegistryStateDBOption(builder.cs.registry))
		}
		opts := []factory.StateDBOption{
			factory.RegistryStateDBOption(builder.cs.registry),
			factory.DefaultPatchOption(),
		}
		if builder.cfg.Chain.EnableStateDBCaching {
			dao, err = db.CreateKVStoreWithCache(builder.cfg.DB, builder.cfg.Chain.TrieDBPath, builder.cfg.Chain.StateDBCacheSize)
		} else {
			dao, err = db.CreateKVStore(builder.cfg.DB, builder.cfg.Chain.TrieDBPath)
		}
		if err != nil {
			return nil, err
		}
		return factory.NewStateDB(factoryCfg, dao, opts...)
	}
	if forTest {
		return factory.NewFactory(factoryCfg, db.NewMemKVStore(), factory.RegistryOption(builder.cs.registry))
	}
	dao, err = db.CreateKVStore(builder.cfg.DB, builder.cfg.Chain.TrieDBPath)
	if err != nil {
		return nil, err
	}
	return factory.NewFactory(
		factoryCfg,
		dao,
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
	committeeConfig.GravityChainStartHeight = builder.cfg.Genesis.GravityChainStartHeight
	if committeeConfig.GravityChainStartHeight == 0 {
		return nil, nil
	}
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
		ac, err := actpool.NewActPool(builder.cfg.Genesis, builder.cs.factory, builder.cfg.ActPool)
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
	// indexers in synchronizedIndexers will need to run PutBlock() one by one
	// factory is dependent on sgdIndexer and contractStakingIndexer, so it should be put in the first place
	synchronizedIndexers := []blockdao.BlockIndexer{builder.cs.factory}
	if builder.cs.contractStakingIndexer != nil {
		synchronizedIndexers = append(synchronizedIndexers, builder.cs.contractStakingIndexer)
	}
	if builder.cs.sgdIndexer != nil {
		synchronizedIndexers = append(synchronizedIndexers, builder.cs.sgdIndexer)
	}
	if len(synchronizedIndexers) > 1 {
		indexers = append(indexers, blockindex.NewSyncIndexers(synchronizedIndexers...))
	} else {
		indexers = append(indexers, builder.cs.factory)
	}
	if !builder.cfg.Chain.EnableAsyncIndexWrite && builder.cs.indexer != nil {
		indexers = append(indexers, builder.cs.indexer)
	}
	if builder.cs.bfIndexer != nil {
		indexers = append(indexers, builder.cs.bfIndexer)
	}
	if forTest {
		builder.cs.blockdao = blockdao.NewBlockDAOInMemForTest(indexers)
	} else {
		dbConfig := builder.cfg.DB
		dbConfig.DbPath = builder.cfg.Chain.ChainDBPath
		deser := block.NewDeserializer(builder.cfg.Chain.EVMNetworkID)
		builder.cs.blockdao = blockdao.NewBlockDAO(indexers, dbConfig, deser)
	}

	return nil
}

func (builder *Builder) buildSGDRegistry(forTest bool) error {
	if builder.cs.sgdIndexer != nil {
		return nil
	}
	if forTest || builder.cfg.Genesis.SystemSGDContractAddress == "" {
		builder.cs.sgdIndexer = nil
		return nil
	}
	kvStore, err := db.CreateKVStoreWithCache(builder.cfg.DB, builder.cfg.Chain.SGDIndexDBPath, 1000)
	if err != nil {
		return err
	}
	builder.cs.sgdIndexer = blockindex.NewSGDRegistry(builder.cfg.Genesis.SystemSGDContractAddress, builder.cfg.Genesis.SystemSGDContractHeight, kvStore)
	return nil
}

func (builder *Builder) buildContractStakingIndexer(forTest bool) error {
	if !builder.cfg.Chain.EnableStakingProtocol {
		return nil
	}
	if builder.cs.contractStakingIndexer != nil {
		return nil
	}
	if forTest || builder.cfg.Genesis.SystemStakingContractAddress == "" {
		builder.cs.contractStakingIndexer = nil
		return nil
	}
	dbConfig := builder.cfg.DB
	dbConfig.DbPath = builder.cfg.Chain.ContractStakingIndexDBPath
	voteCalcConsts := builder.cfg.Genesis.VoteWeightCalConsts
	indexer, err := contractstaking.NewContractStakingIndexer(
		db.NewBoltDB(dbConfig),
		contractstaking.Config{
			ContractAddress:      builder.cfg.Genesis.SystemStakingContractAddress,
			ContractDeployHeight: builder.cfg.Genesis.SystemStakingContractHeight,
			CalculateVoteWeight: func(v *staking.VoteBucket) *big.Int {
				return staking.CalculateVoteWeight(voteCalcConsts, v, false)
			},
			BlockInterval: builder.cfg.DardanellesUpgrade.BlockInterval,
		})
	if err != nil {
		return err
	}
	builder.cs.contractStakingIndexer = indexer
	return nil
}

func (builder *Builder) buildGatewayComponents(forTest bool) error {
	indexer, bfIndexer, candidateIndexer, candBucketsIndexer, err := builder.createGateWayComponents(forTest)
	if err != nil {
		return errors.Wrapf(err, "failed to create gateway components")
	}
	builder.cs.candidateIndexer = candidateIndexer
	if builder.cs.candidateIndexer != nil {
		builder.cs.lifecycle.Add(builder.cs.candidateIndexer)
	}
	builder.cs.candBucketsIndexer = candBucketsIndexer
	if builder.cs.candBucketsIndexer != nil {
		builder.cs.lifecycle.Add(builder.cs.candBucketsIndexer)
	}
	builder.cs.bfIndexer = bfIndexer
	builder.cs.indexer = indexer

	return nil
}

func (builder *Builder) createGateWayComponents(forTest bool) (
	indexer blockindex.Indexer,
	bfIndexer blockindex.BloomFilterIndexer,
	candidateIndexer *poll.CandidateIndexer,
	candBucketsIndexer *staking.CandidatesBucketsIndexer,
	err error,
) {
	_, gateway := builder.cfg.Plugins[config.GatewayPlugin]
	if !gateway {
		return
	}

	if forTest {
		indexer, err = blockindex.NewIndexer(db.NewMemKVStore(), builder.cfg.Genesis.Hash())
		if err != nil {
			return
		}
		bfIndexer, err = blockindex.NewBloomfilterIndexer(db.NewMemKVStore(), builder.cfg.Indexer)
		if err != nil {
			return
		}
		candidateIndexer, err = poll.NewCandidateIndexer(db.NewMemKVStore())
		if err != nil {
			return
		}
		if builder.cfg.Chain.EnableStakingIndexer {
			candBucketsIndexer, err = staking.NewStakingCandidatesBucketsIndexer(db.NewMemKVStore())
		}
		return
	}
	dbConfig := builder.cfg.DB
	dbConfig.DbPath = builder.cfg.Chain.IndexDBPath
	indexer, err = blockindex.NewIndexer(db.NewBoltDB(dbConfig), builder.cfg.Genesis.Hash())
	if err != nil {
		return
	}

	// create bloomfilter indexer
	dbConfig.DbPath = builder.cfg.Chain.BloomfilterIndexDBPath
	bfIndexer, err = blockindex.NewBloomfilterIndexer(db.NewBoltDB(dbConfig), builder.cfg.Indexer)
	if err != nil {
		return
	}

	// create candidate indexer
	dbConfig.DbPath = builder.cfg.Chain.CandidateIndexDBPath
	candidateIndexer, err = poll.NewCandidateIndexer(db.NewBoltDB(dbConfig))
	if err != nil {
		return
	}

	// create staking indexer
	if builder.cfg.Chain.EnableStakingIndexer {
		dbConfig.DbPath = builder.cfg.Chain.StakingIndexDBPath
		candBucketsIndexer, err = staking.NewStakingCandidatesBucketsIndexer(db.NewBoltDB(dbConfig))
	}
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
		indexBuilder, err := blockindex.NewIndexBuilder(builder.cs.chain.ChainID(), builder.cfg.Genesis, builder.cs.blockdao, builder.cs.indexer)
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

	return blockchain.NewBlockchain(builder.cfg.Chain, builder.cfg.Genesis, builder.cs.blockdao, factory.NewMinter(builder.cs.factory, builder.cs.actpool), chainOpts...)
}

func (builder *Builder) buildNodeInfoManager() error {
	cs := builder.cs
	stk := staking.FindProtocol(cs.Registry())
	if stk == nil {
		return errors.New("cannot find staking protocol")
	}
	chain := builder.cs.chain
	dm := nodeinfo.NewInfoManager(&builder.cfg.NodeInfo, cs.p2pAgent, cs.chain, builder.cfg.Chain.ProducerPrivateKey(), func() []string {
		ctx := protocol.WithFeatureCtx(
			protocol.WithBlockCtx(
				genesis.WithGenesisContext(context.Background(), chain.Genesis()),
				protocol.BlockCtx{BlockHeight: chain.TipHeight()},
			),
		)
		candidates, err := stk.ActiveCandidates(ctx, cs.factory, 0)
		if err != nil {
			log.L().Error("failed to get active candidates", zap.Error(errors.WithStack(err)))
			return nil
		}
		whiteList := make([]string, len(candidates))
		for i := range whiteList {
			whiteList[i] = candidates[i].Address
		}
		return whiteList
	})
	builder.cs.nodeInfoManager = dm
	builder.cs.lifecycle.Add(dm)
	return nil
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
		p2pAgent.ConnectedPeers,
		p2pAgent.UnicastOutbound,
		p2pAgent.BlockPeer,
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

	stakingProtocol, err := staking.NewProtocol(
		rewarding.DepositGas,
		&staking.BuilderConfig{
			Staking:                  builder.cfg.Genesis.Staking,
			PersistStakingPatchBlock: builder.cfg.Chain.PersistStakingPatchBlock,
			StakingPatchDir:          builder.cfg.Chain.StakingPatchDir,
		},
		builder.cs.candBucketsIndexer,
		builder.cs.contractStakingIndexer,
		builder.cfg.Genesis.OkhotskBlockHeight,
		builder.cfg.Genesis.GreenlandBlockHeight,
		builder.cfg.Genesis.HawaiiBlockHeight,
	)
	if err != nil {
		return err
	}

	return stakingProtocol.Register(builder.cs.registry)
}

func (builder *Builder) registerRewardingProtocol() error {
	// TODO: rewarding protocol for standalone mode is weird, rDPoSProtocol could be passed via context
	return rewarding.NewProtocol(builder.cfg.Genesis.Rewarding).Register(builder.cs.registry)
}

func (builder *Builder) registerAccountProtocol() error {
	return account.NewProtocol(rewarding.DepositGas).Register(builder.cs.registry)
}

func (builder *Builder) registerExecutionProtocol() error {
	return execution.NewProtocol(builder.cs.blockdao.GetBlockHash, rewarding.DepositGasWithSGD, builder.cs.sgdIndexer, builder.cs.blockTimeCalculator.CalculateBlockTime).Register(builder.cs.registry)
}

func (builder *Builder) registerRollDPoSProtocol() error {
	if builder.cfg.Consensus.Scheme != config.RollDPoSScheme {
		return nil
	}
	if err := rolldpos.NewProtocol(
		builder.cfg.Genesis.NumCandidateDelegates,
		builder.cfg.Genesis.NumDelegates,
		builder.cfg.Genesis.NumSubEpochs,
		rolldpos.EnableDardanellesSubEpoch(builder.cfg.Genesis.DardanellesBlockHeight, builder.cfg.Genesis.DardanellesNumSubEpochs),
	).Register(builder.cs.registry); err != nil {
		return err
	}
	factory := builder.cs.factory
	dao := builder.cs.blockdao
	chain := builder.cs.chain
	getBlockTime := builder.cs.blockTimeCalculator.CalculateBlockTime
	pollProtocol, err := poll.NewProtocol(
		builder.cfg.Consensus.Scheme,
		builder.cfg.Chain,
		builder.cfg.Genesis,
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

			// TODO: add depositGas
			ctx = evm.WithHelperCtx(ctx, evm.HelperContext{
				GetBlockHash: dao.GetBlockHash,
				GetBlockTime: getBlockTime,
			})
			data, _, err := factory.SimulateExecution(ctx, addr, ex)

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
		dao.GetBlockHash,
		getBlockTime,
	)
	if err != nil {
		return errors.Wrap(err, "failed to generate poll protocol")
	}
	return pollProtocol.Register(builder.cs.registry)
}

func (builder *Builder) buildBlockTimeCalculator() (err error) {
	consensusCfg := consensusfsm.NewConsensusConfig(builder.cfg.Consensus.RollDPoS.FSM, builder.cfg.DardanellesUpgrade, builder.cfg.Genesis, builder.cfg.Consensus.RollDPoS.Delay)
	dao := builder.cs.BlockDAO()
	builder.cs.blockTimeCalculator, err = blockutil.NewBlockTimeCalculator(consensusCfg.BlockInterval, builder.cs.Blockchain().TipHeight, func(height uint64) (time.Time, error) {
		blk, err := dao.GetBlockByHeight(height)
		if err != nil {
			return time.Time{}, err
		}
		return blk.Timestamp(), nil
	})
	return err
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
	builderCfg := rp.BuilderConfig{
		Chain:              builder.cfg.Chain,
		Consensus:          builder.cfg.Consensus.RollDPoS,
		Scheme:             builder.cfg.Consensus.Scheme,
		DardanellesUpgrade: builder.cfg.DardanellesUpgrade,
		DB:                 builder.cfg.DB,
		Genesis:            builder.cfg.Genesis,
		SystemActive:       builder.cfg.System.Active,
	}
	component, err := consensus.NewConsensus(builderCfg, builder.cs.chain, builder.cs.factory, copts...)
	if err != nil {
		return errors.Wrap(err, "failed to create consensus component")
	}
	builder.cs.consensus = component
	builder.cs.lifecycle.Add(component)

	return nil
}

func (builder *Builder) build(forSubChain, forTest bool) (*ChainService, error) {
	builder.cs.registry = protocol.NewRegistry()
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
	if err := builder.buildSGDRegistry(forTest); err != nil {
		return nil, err
	}
	if err := builder.buildContractStakingIndexer(forTest); err != nil {
		return nil, err
	}
	if err := builder.buildBlockDAO(forTest); err != nil {
		return nil, err
	}
	if err := builder.buildBlockchain(forSubChain, forTest); err != nil {
		return nil, err
	}
	if err := builder.buildBlockTimeCalculator(); err != nil {
		return nil, err
	}
	// staking protocol need to be put in registry before poll protocol when enabling
	if err := builder.registerStakingProtocol(); err != nil {
		return nil, errors.Wrap(err, "failed to register staking protocol")
	}
	if err := builder.registerAccountProtocol(); err != nil {
		return nil, errors.Wrap(err, "failed to register rewarding protocol")
	}
	if err := builder.registerRollDPoSProtocol(); err != nil {
		return nil, errors.Wrap(err, "failed to register roll dpos related protocols")
	}
	if err := builder.registerExecutionProtocol(); err != nil {
		return nil, errors.Wrap(err, "failed to register execution protocol")
	}
	if err := builder.registerRewardingProtocol(); err != nil {
		return nil, errors.Wrap(err, "failed to register rewarding protocol")
	}
	if err := builder.buildConsensusComponent(); err != nil {
		return nil, err
	}
	if err := builder.buildBlockSyncer(); err != nil {
		return nil, err
	}
	if err := builder.buildNodeInfoManager(); err != nil {
		return nil, err
	}
	cs := builder.cs
	builder.cs = nil

	return cs, nil
}
