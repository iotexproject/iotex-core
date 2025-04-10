// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package consensus

import (
	"context"

	"github.com/facebookgo/clock"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/poll"
	rp "github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/consensus/scheme"
	"github.com/iotexproject/iotex-core/v2/consensus/scheme/rolldpos"
	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
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
	cfg    Config
	scheme scheme.Scheme
}

type optionParams struct {
	broadcastHandler scheme.Broadcast
	pp               poll.Protocol
	rp               *rp.Protocol
	bbf              rolldpos.BlockBuilderFactory
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

// WithBlockBuilderFactory is an option to set block builder factory
func WithBlockBuilderFactory(bbf rolldpos.BlockBuilderFactory) Option {
	return func(ops *optionParams) error {
		ops.bbf = bbf
		return nil
	}
}

// NewConsensus creates a IotxConsensus struct.
func NewConsensus(
	cfg rolldpos.BuilderConfig,
	bc blockchain.Blockchain,
	sf rolldpos.StateReaderFactory,
	opts ...Option,
) (Consensus, error) {
	var ops optionParams
	for _, opt := range opts {
		if err := opt(&ops); err != nil {
			return nil, err
		}
	}

	clock := clock.New()
	cs := &IotxConsensus{cfg: Config{
		Scheme:   cfg.Scheme,
		RollDPoS: cfg.Consensus,
	}}
	var err error
	switch cfg.Scheme {
	case RollDPoSScheme:
		if ops.bbf == nil {
			return nil, errors.New("block builder factory is not set")
		}
		chainMgr := rolldpos.NewChainManager(bc, sf, ops.bbf)
		delegatesByEpochFunc := func(epochNum uint64, prevHash []byte) ([]string, error) {
			fork, err := chainMgr.Fork(hash.Hash256(prevHash))
			if err != nil {
				return nil, err
			}
			forkSF, err := fork.StateReader()
			if err != nil {
				return nil, err
			}
			re := protocol.NewRegistry()
			if err := ops.rp.Register(re); err != nil {
				return nil, err
			}
			ctx := genesis.WithGenesisContext(
				protocol.WithRegistry(context.Background(), re),
				cfg.Genesis,
			)
			ctx = protocol.WithFeatureWithHeightCtx(ctx)
			tipHeight := fork.TipHeight()
			tipEpochNum := ops.rp.GetEpochNum(tipHeight)
			var candidatesList state.CandidateList
			switch epochNum {
			case tipEpochNum:
				candidatesList, err = ops.pp.Delegates(ctx, forkSF)
			case tipEpochNum + 1:
				candidatesList, err = ops.pp.NextDelegates(ctx, forkSF)
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
		}
		proposersByEpochFunc := delegatesByEpochFunc
		bd := rolldpos.NewRollDPoSBuilder().
			SetPriKey(cfg.Chain.ProducerPrivateKeys()...).
			SetConfig(cfg).
			SetChainManager(chainMgr).
			SetBlockDeserializer(block.NewDeserializer(bc.EvmNetworkID())).
			SetClock(clock).
			SetBroadcast(ops.broadcastHandler).
			SetDelegatesByEpochFunc(delegatesByEpochFunc).
			SetProposersByEpochFunc(proposersByEpochFunc).
			RegisterProtocol(ops.rp)
		// TODO: explorer dependency deleted here at #1085, need to revive by migrating to api
		cs.scheme, err = bd.Build()
		if err != nil {
			log.Logger("consensus").Panic("Error when constructing RollDPoS.", zap.Error(err))
		}
	case NOOPScheme:
		cs.scheme = scheme.NewNoop()
	case StandaloneScheme:
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
		return nil, errors.Errorf("unexpected IotxConsensus scheme %s", cfg.Scheme)
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
