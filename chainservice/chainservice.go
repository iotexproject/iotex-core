// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package chainservice

import (
	"context"
	"math/rand"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"go.uber.org/zap"

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
	api                *api.ServerV2
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
	g := cs.chain.Genesis()
	act, err := (&action.Deserializer{}).WithChainID(g.IsNewfoundland(cs.chain.TipHeight())).ActionToSealedEnvelope(actPb)
	if err != nil {
		return err
	}
	ctx = protocol.WithRegistry(ctx, cs.registry)
	err = cs.actpool.Add(ctx, act)
	if err != nil {
		log.L().Debug(err.Error())
	}
	return err
}

// HandleBlock handles incoming block request.
func (cs *ChainService) HandleBlock(ctx context.Context, peer string, pbBlock *iotextypes.Block) error {
	g := cs.chain.Genesis()
	blk, err := (&block.Deserializer{}).WithChainID(g.IsNewfoundland(pbBlock.Header.Core.Height)).FromBlockProto(pbBlock)
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

// APIServer returns the grpc server
func (cs *ChainService) APIServer() *api.GRPCServer {
	return cs.api.GrpcServer
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

// TODO: replace isDummyBlockSyncer by declaring BlockSyncerMode in blocksyncer package
func createBlockSyncer(
	isDummyBlockSyncer bool,
	cfgBS config.BlockSync,
	cons consensus.Consensus,
	chain blockchain.Blockchain,
	p2pAgent p2p.Agent,
	dao blockdao.BlockDAO,
	isHawaiiHeightHandler func(uint64) bool,
) (blocksync.BlockSync, error) {
	if isDummyBlockSyncer {
		return blocksync.NewDummyBlockSyncer(), nil
	}
	return blocksync.NewBlockSyncer(
		cfgBS,
		chain.TipHeight,
		func(height uint64) (*block.Block, error) {
			return dao.GetBlockByHeight(height)
		},
		func(blk *block.Block) error {
			if err := cons.ValidateBlockFooter(blk); err != nil {
				log.L().Debug("Failed to validate block footer.", zap.Error(err), zap.Uint64("height", blk.Height()))
				return err
			}
			retries := 1
			if !isHawaiiHeightHandler(blk.Height()) {
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
			cons.Calibrate(blk.Height())
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
				return
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
}
