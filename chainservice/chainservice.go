// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package chainservice

import (
	"context"
	"strconv"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-election/committee"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/api"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state/factory"
)

var (
	_apiCallWithChainIDMtc = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_apicall_chainid_metrics",
			Help: "API call ChainID Statistics",
		},
		[]string{"chain_id"},
	)
	_apiCallWithOutChainIDMtc = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_apicall_nochainid_metrics",
			Help: "API call Without ChainID Statistics",
		},
		[]string{"sender", "recipient"},
	)
)

func init() {
	prometheus.MustRegister(_apiCallWithChainIDMtc)
	prometheus.MustRegister(_apiCallWithOutChainIDMtc)
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
	indexer            blockindex.Indexer
	bfIndexer          blockindex.BloomFilterIndexer
	candidateIndexer   *poll.CandidateIndexer
	candBucketsIndexer *staking.CandidatesBucketsIndexer
	registry           *protocol.Registry
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
func (cs *ChainService) ReportFullness(_ context.Context, _ iotexrpc.MessageType, fullness float32) {
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
	chainIDmetrics(act)
	return err
}

func chainIDmetrics(act action.SealedEnvelope) {
	chainID := strconv.FormatUint(uint64(act.ChainID()), 10)
	if act.ChainID() > 0 {
		_apiCallWithChainIDMtc.WithLabelValues(chainID).Inc()
	} else {
		recipient, _ := act.Destination()
		//it will be empty for staking action, change string to staking in such case
		if recipient == "" {
			act, ok := act.Action().(action.EthCompatibleAction)
			if ok {
				if ethTx, err := act.ToEthTx(); err == nil && ethTx.To() != nil {
					if add, err := address.FromHex(ethTx.To().Hex()); err == nil {
						recipient = add.String()
					}
				}
			}
			if recipient == "" {
				recipient = "staking"
			}
		}
		_apiCallWithOutChainIDMtc.WithLabelValues(act.SenderAddress().String(), recipient).Inc()
	}
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

// Registry returns a pointer to the registry
func (cs *ChainService) Registry() *protocol.Registry { return cs.registry }

// NewAPIServer creates a new api server
func (cs *ChainService) NewAPIServer(cfg config.API, scheme string) (*api.ServerV2, error) {
	if cfg.GRPCPort == 0 && cfg.HTTPPort == 0 {
		return nil, nil
	}
	p2pAgent := cs.p2pAgent
	apiServerOptions := []api.Option{
		api.WithBroadcastOutbound(func(ctx context.Context, chainID uint32, msg proto.Message) error {
			return p2pAgent.BroadcastOutbound(ctx, msg)
		}),
		api.WithNativeElection(cs.electionCommittee),
	}

	svr, err := api.NewServerV2(
		cfg,
		scheme,
		cs.chain,
		cs.blocksync,
		cs.factory,
		cs.blockdao,
		cs.indexer,
		cs.bfIndexer,
		cs.actpool,
		cs.registry,
		apiServerOptions...,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create API server")
	}

	return svr, nil
}
