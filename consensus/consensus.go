// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package consensus

import (
	"context"

	"github.com/facebookgo/clock"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	rp "github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/consensus/scheme/rolldpos"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// Consensus is the interface for handling IotxConsensus view change.
type Consensus interface {
	lifecycle.StartStopper

	HandleConsensusMsg(*iotextypes.ConsensusMessage) error
	Calibrate(uint64)
	ValidateBlockFooter(*block.Block) error
	Metrics() (scheme.ConsensusMetrics, error)
	Activate(bool)
	Active() bool
}

// IotxConsensus implements Consensus
type IotxConsensus struct {
	cfg    config.Consensus
	scheme scheme.Scheme
}

type optionParams struct {
	broadcastHandler scheme.Broadcast
	pp               poll.Protocol
	rp               *rp.Protocol
}

// Option sets Consensus construction parameter.
type Option func(op *optionParams) error

// WithBroadcast is an option to add broadcast callback to Consensus
func WithBroadcast(broadcastHandler scheme.Broadcast) Option {
	return func(ops *optionParams) error {
		ops.broadcastHandler = broadcastHandler
		return nil
	}
}

// WithRollDPoSProtocol is an option to register rolldpos protocol
func WithRollDPoSProtocol(rp *rp.Protocol) Option {
	return func(ops *optionParams) error {
		ops.rp = rp
		return nil
	}
}

// WithPollProtocol is an option to register poll protocol
func WithPollProtocol(pp poll.Protocol) Option {
	return func(ops *optionParams) error {
		ops.pp = pp
		return nil
	}
}

// NewConsensus creates a IotxConsensus struct.
func NewConsensus(
	cfg config.Config,
	bc blockchain.Blockchain,
	sf factory.Factory,
	opts ...Option,
) (Consensus, error) {
	var ops optionParams
	for _, opt := range opts {
		if err := opt(&ops); err != nil {
			return nil, err
		}
	}

	clock := clock.New()
	cs := &IotxConsensus{cfg: cfg.Consensus}
	var err error
	switch cfg.Consensus.Scheme {
	case config.RollDPoSScheme:
		bd := rolldpos.NewRollDPoSBuilder().
			SetAddr(cfg.Chain.ProducerAddress().String()).
			SetPriKey(cfg.Chain.ProducerPrivateKey()).
			SetConfig(cfg).
			SetChainManager(rolldpos.NewChainManager(bc)).
			SetBlockDeserializer(block.NewDeserializer(bc.EvmNetworkID())).
			SetClock(clock).
			SetBroadcast(ops.broadcastHandler).
			SetDelegatesByEpochFunc(func(epochNum uint64) ([]string, error) {
				re := protocol.NewRegistry()
				if err := ops.rp.Register(re); err != nil {
					return nil, err
				}
				ctx := genesis.WithGenesisContext(
					protocol.WithRegistry(context.Background(), re),
					cfg.Genesis,
				)
				ctx = protocol.WithFeatureWithHeightCtx(ctx)
				tipHeight := bc.TipHeight()
				tipEpochNum := ops.rp.GetEpochNum(tipHeight)
				var candidatesList state.CandidateList
				var err error
				switch epochNum {
				case tipEpochNum:
					candidatesList, err = ops.pp.Delegates(ctx, sf)
				case tipEpochNum + 1:
					candidatesList, err = ops.pp.NextDelegates(ctx, sf)
				default:
					err = errors.Errorf("invalid epoch number %d compared to tip epoch number %d", epochNum, tipEpochNum)
				}
				if err != nil {
					return nil, err
				}
				addrs := []string{}
				for _, candidate := range candidatesList {
					addrs = append(addrs, candidate.Address)
				}
				return addrs, nil
			}).
			RegisterProtocol(ops.rp)
		// TODO: explorer dependency deleted here at #1085, need to revive by migrating to api
		cs.scheme, err = bd.Build()
		if err != nil {
			log.Logger("consensus").Panic("Error when constructing RollDPoS.", zap.Error(err))
		}
	case config.NOOPScheme:
		cs.scheme = scheme.NewNoop()
	case config.StandaloneScheme:
		mintBlockCB := func() (*block.Block, error) {
			blk, err := bc.MintNewBlock(clock.Now())
			if err != nil {
				log.Logger("consensus").Error("Failed to mint a block.", zap.Error(err))
				return nil, err
			}
			log.Logger("consensus").Info("Created a new block.",
				zap.Uint64("height", blk.Height()),
				zap.Int("length", len(blk.Actions)))
			return blk, nil
		}
		commitBlockCB := func(blk *block.Block) error {
			err := bc.CommitBlock(blk)
			if err != nil {
				log.Logger("consensus").Error("Failed to commit the block.", zap.Error(err), zap.Uint64("height", blk.Height()))
			}
			return err
		}
		broadcastBlockCB := func(blk *block.Block) error {
			if blkPb := blk.ConvertToBlockPb(); blkPb != nil {
				return ops.broadcastHandler(blkPb)
			}
			return nil
		}
		cs.scheme = scheme.NewStandalone(
			mintBlockCB,
			commitBlockCB,
			broadcastBlockCB,
			bc,
			cfg.Genesis.BlockInterval,
		)
	default:
		return nil, errors.Errorf("unexpected IotxConsensus scheme %s", cfg.Consensus.Scheme)
	}

	return cs, nil
}

// Start starts running the consensus algorithm
func (c *IotxConsensus) Start(ctx context.Context) error {
	log.Logger("consensus").Info("Starting IotxConsensus scheme.", zap.String("scheme", c.cfg.Scheme))

	err := c.scheme.Start(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to start scheme %s", c.cfg.Scheme)
	}
	return nil
}

// Stop stops running the consensus algorithm
func (c *IotxConsensus) Stop(ctx context.Context) error {
	log.Logger("consensus").Info("Stopping IotxConsensus scheme.", zap.String("scheme", c.cfg.Scheme))

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

// HandleConsensusMsg handles consensus messages
func (c *IotxConsensus) HandleConsensusMsg(msg *iotextypes.ConsensusMessage) error {
	return c.scheme.HandleConsensusMsg(msg)
}

// Calibrate triggers an event to calibrate consensus context
func (c *IotxConsensus) Calibrate(height uint64) {
	c.scheme.Calibrate(height)
}

// ValidateBlockFooter validates the signatures in block footer
func (c *IotxConsensus) ValidateBlockFooter(blk *block.Block) error {
	return c.scheme.ValidateBlockFooter(blk)
}

// Scheme returns the scheme instance
func (c *IotxConsensus) Scheme() scheme.Scheme {
	return c.scheme
}

// Activate activates or pauses the consensus component
func (c *IotxConsensus) Activate(active bool) {
	c.scheme.Activate(active)
}

// Active returns true if the consensus component is active or false if it stands by
func (c *IotxConsensus) Active() bool { return c.scheme.Active() }
