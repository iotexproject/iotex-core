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
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/consensus/scheme/rolldpos"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
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
	p2p network.Overlay,
) Consensus {
	if bc == nil || ap == nil || p2p == nil {
		logger.Panic().Msg("Try to attach to nil blockchain, action pool or p2p interface")
	}

	cs := &IotxConsensus{cfg: &cfg.Consensus}
	mintBlockCB := func() (*blockchain.Block, error) {
		transfers, votes, executions := ap.PickActs()
		logger.Debug().
			Int("transfer", len(transfers)).
			Int("votes", len(votes)).
			Int("Executions", len(executions)).
			Msg("pick actions")
		addr, err := cfg.ProducerAddr()
		if err != nil {
			return nil, err
		}
		blk, err := bc.MintNewBlock(transfers, votes, executions, addr, "")
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

	commitBlockCB := func(blk *blockchain.Block) error {
		err := bc.CommitBlock(blk)
		if err != nil {
			logger.Error().Err(err).Int64("Height", int64(blk.Height())).Msg("Failed to commit the block")
		}
		// Remove transfers in this block from ActPool and reset ActPool state
		ap.Reset()
		return err
	}

	broadcastBlockCB := func(blk *blockchain.Block) error {
		if blkPb := blk.ConvertToBlockPb(); blkPb != nil {
			return p2p.Broadcast(blkPb)
		}
		return nil
	}

	addr, err := cfg.ProducerAddr()
	if err != nil {
		logger.Panic().Err(err).Msg("Fail to create new consensus")
	}

	switch cfg.Consensus.Scheme {
	case config.RollDPoSScheme:
		cs.scheme, err = rolldpos.NewRollDPoSBuilder().
			SetAddr(addr).
			SetConfig(cfg.Consensus.RollDPoS).
			SetBlockchain(bc).
			SetActPool(ap).
			SetP2P(p2p).
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
