// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package consensus

import (
	"context"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/consensus/scheme/rolldpos"
	"github.com/iotexproject/iotex-core/consensus/scheme/rolldpos2"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/errcode"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
)

// Consensus is the interface for handling IotxConsensus view change.
type Consensus interface {
	lifecycle.StartStopper

	HandleViewChange(proto.Message, chan bool) error
	HandleBlockPropose(proto.Message, chan bool) error
	Metrics() (scheme.ConsensusMetrics, error)
}

// IotxConsensus implements Consensus
type IotxConsensus struct {
	cfg    *config.Consensus
	scheme scheme.Scheme
}

// NewConsensus creates a IotxConsensus struct.
func NewConsensus(
	cfg *config.Config,
	bc blockchain.Blockchain,
	ap actpool.ActPool,
	bs blocksync.BlockSync,
	dlg delegate.Pool,
) Consensus {
	if bc == nil || bs == nil {
		logger.Error().Msg("Try to attach to chain or bs == nil")
		return nil
	}

	cs := &IotxConsensus{cfg: &cfg.Consensus}
	mintBlockCB := func() (*blockchain.Block, error) {
		transfers, votes := ap.PickActs()
		logger.Debug().
			Int("transfer", len(transfers)).
			Int("votes", len(votes)).
			Msg("pick actions")
		addr, err := cfg.ProducerAddr()
		if err != nil {
			return nil, err
		}
		blk, err := bc.MintNewBlock(transfers, votes, addr, "")
		if err != nil {
			logger.Error().Msg("Failed to mint a block")
			return nil, err
		}
		logger.Info().
			Uint64("height", blk.Height()).
			Int("length", len(blk.Transfers)).
			Msg("created a new block")
		return blk, nil
	}

	_ = func(msg proto.Message) error {
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

	addr, err := cfg.ProducerAddr()
	if err != nil {
		logger.Panic().Err(err).Msg("Fail to create new consensus")
	}

	switch cfg.Consensus.Scheme {
	case config.RollDPoSScheme:
		/*
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
				addr.RawAddress,
				dlg,
			)
		*/
		cs.scheme, err = rolldpos2.NewRollDPoSBuilder().
			SetAddr(addr).
			SetConfig(cfg.Consensus.RollDPoS).
			SetBlockchain(bc).
			SetActPool(ap).
			SetP2P(bs.P2P()).
			Build()
		if err != nil {
			logger.Panic().Err(err).Msg("error when constructing RollDPoS")
		}
	case config.NOOPScheme:
		cs.scheme = scheme.NewNoop()
	case config.StandaloneScheme:
		cs.scheme = scheme.NewStandalone(
			mintBlockCB,
			commitBlockCB,
			broadcastBlockCB,
			bc,
			cfg.Consensus.BlockCreationInterval,
		)
	default:
		logger.Error().
			Str("scheme", cfg.Consensus.Scheme).
			Msg("Unexpected IotxConsensus scheme")
		return nil
	}

	return cs
}

// Start starts running the consensus algorithm
func (c *IotxConsensus) Start(ctx context.Context) error {
	logger.Info().
		Str("scheme", c.cfg.Scheme).
		Msg("Starting IotxConsensus scheme")

	err := c.scheme.Start(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to start scheme %s", c.cfg.Scheme)
	}
	return nil
}

// Stop stops running the consensus algorithm
func (c *IotxConsensus) Stop(ctx context.Context) error {
	logger.Info().
		Str("scheme", c.cfg.Scheme).
		Msg("Stopping IotxConsensus scheme")

	err := c.scheme.Stop(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to stop scheme %s", c.cfg.Scheme)
	}
	return nil
}

// Metrics returns consensus metrics
func (c *IotxConsensus) Metrics() (scheme.ConsensusMetrics, error) {
	return c.scheme.Metrics()
}

// HandleViewChange dispatches the call to different schemes
func (c *IotxConsensus) HandleViewChange(m proto.Message, done chan bool) error {
	return c.scheme.Handle(m)
}

// HandleBlockPropose handles a proposed block
func (c *IotxConsensus) HandleBlockPropose(m proto.Message, done chan bool) error {
	return errcode.ErrNotImplemented
}

// Scheme returns the scheme instance
func (c *IotxConsensus) Scheme() scheme.Scheme {
	return c.scheme
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
	case "PseudoStartRollingEpoch":
		epochCb = rolldpos.PseudoStartRollingEpoch
	case "StartRollingEpoch":
		epochCb = rolldpos.StartRollingEpoch
	default:
		logger.Panic().
			Str("func name", epochCbName).
			Msg("invalid StartNextEpochCB implementation")
	}
	return
}
