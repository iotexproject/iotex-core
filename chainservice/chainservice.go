// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package chainservice

import (
	"context"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-election/committee"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/poll"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/actsync"
	"github.com/iotexproject/iotex-core/v2/api"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/v2/blockindex"
	"github.com/iotexproject/iotex-core/v2/blockindex/contractstaking"
	"github.com/iotexproject/iotex-core/v2/blocksync"
	"github.com/iotexproject/iotex-core/v2/consensus"
	"github.com/iotexproject/iotex-core/v2/nodeinfo"
	"github.com/iotexproject/iotex-core/v2/p2p"
	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/server/itx/nodestats"
	"github.com/iotexproject/iotex-core/v2/state/factory"
	"github.com/iotexproject/iotex-core/v2/systemcontractindex/stakingindex"
)

var (
	_blockchainFullnessMtc = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iotex_blockchain_fullness",
			Help: "Blockchain fullness statistics",
		},
		[]string{"message_type"},
	)
)

func init() {
	prometheus.MustRegister(_blockchainFullnessMtc)
}

// ChainService is a blockchain service with all blockchain components.
type ChainService struct {
	lifecycle         lifecycle.Lifecycle
	actpool           actpool.ActPool
	blocksync         blocksync.BlockSync
	consensus         consensus.Consensus
	chain             blockchain.Blockchain
	factory           factory.Factory
	blockdao          blockdao.BlockDAO
	p2pAgent          p2p.Agent
	electionCommittee committee.Committee
	// TODO: explorer dependency deleted at #1085, need to api related params
	indexer                  blockindex.Indexer
	bfIndexer                blockindex.BloomFilterIndexer
	candidateIndexer         *poll.CandidateIndexer
	candBucketsIndexer       *staking.CandidatesBucketsIndexer
	contractStakingIndexer   *contractstaking.Indexer
	contractStakingIndexerV2 stakingindex.StakingIndexer
	contractStakingIndexerV3 stakingindex.StakingIndexer
	registry                 *protocol.Registry
	nodeInfoManager          *nodeinfo.InfoManager
	apiStats                 *nodestats.APILocalStats
	actionsync               *actsync.ActionSync
	minter                   *factory.Minter
}

// Start starts the server
func (cs *ChainService) Start(ctx context.Context) error {
	return cs.lifecycle.OnStartSequentially(ctx)
}

// Stop stops the server
func (cs *ChainService) Stop(ctx context.Context) error {
	return cs.lifecycle.OnStopSequentially(ctx)
}

// ReportFullness switch on or off block sync
func (cs *ChainService) ReportFullness(_ context.Context, messageType iotexrpc.MessageType, fullness float32) {
	_blockchainFullnessMtc.WithLabelValues(iotexrpc.MessageType_name[int32(messageType)]).Set(float64(fullness))
}

// HandleAction handles incoming action request.
func (cs *ChainService) HandleAction(ctx context.Context, actPb *iotextypes.Action) error {
	act, err := (&action.Deserializer{}).SetEvmNetworkID(cs.chain.EvmNetworkID()).ActionToSealedEnvelope(actPb)
	if err != nil {
		return err
	}
	ctx = protocol.WithRegistry(ctx, cs.registry)
	err = cs.actpool.Add(ctx, act)
	if err != nil {
		log.L().Debug(err.Error())
	}
	// TODO: only update action sync for blob action
	hash, err := act.Hash()
	if err != nil {
		return err
	}
	cs.actionsync.ReceiveAction(ctx, hash)
	return nil
}

// HandleActionHash handles incoming action hash request.
func (cs *ChainService) HandleActionHash(ctx context.Context, actHash hash.Hash256, from string) error {
	_, err := cs.actpool.GetActionByHash(actHash)
	if err == nil { // action already in pool
		return nil
	}
	if !errors.Is(err, action.ErrNotFound) {
		return err
	}
	cs.actionsync.RequestAction(ctx, actHash)
	return nil
}

func (cs *ChainService) HandleActionRequest(ctx context.Context, peer peer.AddrInfo, actHash hash.Hash256) error {
	act, err := cs.actpool.GetActionByHash(actHash)
	if err != nil {
		if errors.Is(err, action.ErrNotFound) {
			return nil
		}
		return err
	}
	return cs.p2pAgent.UnicastOutbound(ctx, peer, act.Proto())
}

// HandleBlock handles incoming block request.
func (cs *ChainService) HandleBlock(ctx context.Context, peer string, pbBlock *iotextypes.Block) error {
	blk, err := block.NewDeserializer(cs.chain.EvmNetworkID()).FromBlockProto(pbBlock)
	if err != nil {
		return err
	}
	ctx, err = cs.chain.Context(ctx)
	if err != nil {
		return err
	}
	return cs.blocksync.ProcessBlock(ctx, peer, blk)
}

// HandleSyncRequest handles incoming sync request.
func (cs *ChainService) HandleSyncRequest(ctx context.Context, peer peer.AddrInfo, sync *iotexrpc.BlockSync) error {
	return cs.blocksync.ProcessSyncRequest(ctx, peer, sync.Start, sync.End)
}

// HandleConsensusMsg handles incoming consensus message.
func (cs *ChainService) HandleConsensusMsg(msg *iotextypes.ConsensusMessage) error {
	return cs.consensus.HandleConsensusMsg(msg)
}

// HandleNodeInfo handles nodeinfo message.
func (cs *ChainService) HandleNodeInfo(ctx context.Context, peer string, msg *iotextypes.NodeInfo) error {
	cs.nodeInfoManager.HandleNodeInfo(ctx, peer, msg)
	return nil
}

// HandleNodeInfoRequest handles request node info message
func (cs *ChainService) HandleNodeInfoRequest(ctx context.Context, peer peer.AddrInfo, msg *iotextypes.NodeInfoRequest) error {
	return cs.nodeInfoManager.HandleNodeInfoRequest(ctx, peer)
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

// Consensus returns the consensus instance
func (cs *ChainService) Consensus() consensus.Consensus {
	return cs.consensus
}

// BlockSync returns the block syncer
func (cs *ChainService) BlockSync() blocksync.BlockSync {
	return cs.blocksync
}

// NodeInfoManager returns the delegate manager
func (cs *ChainService) NodeInfoManager() *nodeinfo.InfoManager {
	return cs.nodeInfoManager
}

// Registry returns a pointer to the registry
func (cs *ChainService) Registry() *protocol.Registry { return cs.registry }

// NewAPIServer creates a new api server
func (cs *ChainService) NewAPIServer(cfg api.Config, archive bool) (*api.ServerV2, error) {
	if cfg.GRPCPort == 0 && cfg.HTTPPort == 0 {
		return nil, nil
	}
	p2pAgent := cs.p2pAgent
	apiServerOptions := []api.Option{
		api.WithBroadcastOutbound(func(ctx context.Context, chainID uint32, msg proto.Message) error {
			return p2pAgent.BroadcastOutbound(ctx, msg)
		}),
		api.WithNativeElection(cs.electionCommittee),
		api.WithAPIStats(cs.apiStats),
	}
	if archive {
		apiServerOptions = append(apiServerOptions, api.WithArchiveSupport())
	}

	svr, err := api.NewServerV2(
		cfg,
		cs.chain,
		cs.blocksync,
		cs.factory,
		cs.blockdao,
		cs.indexer,
		cs.bfIndexer,
		cs.actpool,
		cs.registry,
		nil,
		apiServerOptions...,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create API server")
	}

	return svr, nil
}
