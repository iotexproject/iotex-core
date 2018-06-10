// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package consensus

import (
	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/consensus/scheme/rolldpos"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/txpool"
)

// Consensus is the interface for handling consensus view change.
type Consensus interface {
	Start() error
	Stop() error
	HandleViewChange(proto.Message, chan bool) error
	HandleBlockPropose(proto.Message, chan bool) error
}

type consensus struct {
	cfg    *config.Consensus
	scheme scheme.Scheme
}

// NewConsensus creates a consensus struct.
func NewConsensus(cfg *config.Config, bc blockchain.Blockchain, tp txpool.TxPool, ap txpool.ActPool, bs blocksync.BlockSync, dlg delegate.Pool) Consensus {
	if bc == nil || bs == nil {
		logger.Error().Msg("Try to attach to chain or bs == nil")
		return nil
	}

	cs := &consensus{cfg: &cfg.Consensus}
	mintBlockCB := func() (*blockchain.Block, error) {
		// TODO: get list of Transfer and Vote from actPool, instead of nil, nil below
		transfers, err := ap.PickTsfs()
		if err != nil {
			logger.Error().Msg("Failed to pick transfers from actpool")
			return nil, err
		}
		pickedTransfers := []*action.Transfer{}
		for _, tsfs := range transfers {
			pickedTransfers = append(pickedTransfers, tsfs...)
		}
		blk, err := bc.MintNewBlock(tp.PickTxs(), pickedTransfers, nil, &cfg.Chain.MinerAddr, "")
		if err != nil {
			logger.Error().Msg("Failed to mint a block")
			return nil, err
		}
		logger.Info().
			Uint64("height", blk.Height()).
			Int("length", len(blk.Tranxs)).
			Msg("created a new block")
		return blk, nil
	}

	tellBlockCB := func(msg proto.Message) error {
		return bs.P2P().Broadcast(msg)
	}

	commitBlockCB := func(blk *blockchain.Block) error {
		return bs.ProcessBlock(blk)
	}

	broadcastBlockCB := func(blk *blockchain.Block) error {
		if blkPb := blk.ConvertToBlockPb(); blkPb != nil {
			return bs.P2P().Broadcast(blkPb)
		}
		return nil
	}

	switch cfg.Consensus.Scheme {
	case config.RollDPoSScheme:

		cs.scheme = rolldpos.NewRollDPoS(
			cfg.Consensus.RollDPoS,
			mintBlockCB,
			tellBlockCB,
			commitBlockCB,
			broadcastBlockCB,
			chooseGetProposerCB(cfg.Consensus.RollDPoS.ProposerCB),
			chooseStartNextEpochCB(cfg.Consensus.RollDPoS.EpochCB),
			rolldpos.GeneratePseudoDKG,
			bc,
			bs.P2P().Self(),
			dlg)
	case config.NOOPScheme:
		cs.scheme = scheme.NewNoop()
	case config.StandaloneScheme:
		cs.scheme = scheme.NewStandalone(mintBlockCB, commitBlockCB, broadcastBlockCB, bc, cfg.Consensus.BlockCreationInterval)
	default:
		logger.Error().
			Str("scheme", cfg.Consensus.Scheme).
			Msg("Unexpected consensus scheme")
		return nil
	}

	return cs
}

func (c *consensus) Start() error {
	logger.Info().
		Str("scheme", c.cfg.Scheme).
		Msg("Starting consensus scheme")

	c.scheme.Start()
	return nil
}

func (c *consensus) Stop() error {
	logger.Info().
		Str("scheme", c.cfg.Scheme).
		Msg("Stopping consensus scheme")

	c.scheme.Stop()
	return nil
}

// HandleViewChange dispatches the call to different schemes
func (c *consensus) HandleViewChange(m proto.Message, done chan bool) error {
	return c.scheme.Handle(m)
}

// HandleBlockPropose handles a proposed block
func (c *consensus) HandleBlockPropose(m proto.Message, done chan bool) error {
	return common.ErrNotImplemented
}

func chooseGetProposerCB(prCbName string) (prCb scheme.GetProposerCB) {
	switch prCbName {
	case "", "FixedProposer":
		prCb = rolldpos.FixedProposer
	case "PseudoRotatedProposer":
		prCb = rolldpos.PseudoRotatedProposer
	default:
		logger.Panic().
			Str("func name", prCbName).
			Msg("invalid GetProposerCB implementation")
	}
	return
}

func chooseStartNextEpochCB(epochCbName string) (epochCb scheme.StartNextEpochCB) {
	switch epochCbName {
	case "", "NeverStartNewEpoch":
		epochCb = rolldpos.NeverStartNewEpoch
	case "PseudoStarNewEpoch":
		epochCb = rolldpos.PseudoStarNewEpoch
	default:
		logger.Panic().
			Str("func name", epochCbName).
			Msg("invalid StartNextEpochCB implementation")
	}
	return
}
