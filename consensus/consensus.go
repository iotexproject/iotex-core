// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package consensus

import (
	"context"
	"math/big"

	"github.com/facebookgo/clock"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/consensus/scheme/rolldpos"
	explorerapi "github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/state"
)

// Consensus is the interface for handling IotxConsensus view change.
type Consensus interface {
	lifecycle.StartStopper

	HandleBlockPropose(*iproto.ProposePb) error
	HandleEndorse(*iproto.EndorsePb) error
	Metrics() (scheme.ConsensusMetrics, error)
}

// IotxConsensus implements Consensus
type IotxConsensus struct {
	cfg    *config.Consensus
	scheme scheme.Scheme
}

type optionParams struct {
	rootChainAPI explorerapi.Explorer
}

// Option sets Consensus construction parameter.
type Option func(op *optionParams) error

// WithRootChainAPI is an option to add a root chain api to Consensus.
func WithRootChainAPI(exp explorerapi.Explorer) Option {
	return func(ops *optionParams) error {
		ops.rootChainAPI = exp
		return nil
	}
}

// NewConsensus creates a IotxConsensus struct.
func NewConsensus(
	cfg *config.Config,
	bc blockchain.Blockchain,
	ap actpool.ActPool,
	p2p network.Overlay,
	opts ...Option,
) Consensus {
	if bc == nil || ap == nil || p2p == nil {
		logger.Panic().Msg("Try to attach to nil blockchain, action pool or p2p interface")
	}

	var ops optionParams
	for _, opt := range opts {
		if err := opt(&ops); err != nil {
			return nil
		}
	}

	cs := &IotxConsensus{cfg: &cfg.Consensus}
	mintBlockCB := func() (*blockchain.Block, error) {
		acts := ap.PickActs()
		logger.Debug().
			Int("actions", len(acts)).
			Msg("pick actions")

		blk, err := bc.MintNewBlock(acts, GetAddr(cfg), nil,
			nil, "")
		if err != nil {
			logger.Error().Err(err).Msg("Failed to mint a block")
			return nil, err
		}
		logger.Info().
			Uint64("height", blk.Height()).
			Int("length", len(blk.Actions)).
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
			return p2p.Broadcast(bc.ChainID(), blkPb)
		}
		return nil
	}

	var err error
	clock := clock.New()
	switch cfg.Consensus.Scheme {
	case config.RollDPoSScheme:
		bd := rolldpos.NewRollDPoSBuilder().
			SetAddr(GetAddr(cfg)).
			SetConfig(cfg.Consensus.RollDPoS).
			SetBlockchain(bc).
			SetActPool(ap).
			SetClock(clock).
			SetP2P(p2p)
		if ops.rootChainAPI != nil {
			bd = bd.SetCandidatesByHeightFunc(func(h uint64) ([]*state.Candidate, error) {
				rawcs, err := ops.rootChainAPI.GetCandidateMetricsByHeight(int64(h))
				if err != nil {
					return nil, errors.Wrapf(err, "error when get root chain candidates at height %d", h)
				}
				cs := make([]*state.Candidate, 0, len(rawcs.Candidates))
				for _, rawc := range rawcs.Candidates {
					// TODO: this is a short term walk around. We don't need to convert root chain address to sub chain
					// address. Instead we should use public key to identify the block producer
					rootChainAddr, err := address.IotxAddressToAddress(rawc.Address)
					if err != nil {
						return nil, errors.Wrapf(err, "error when get converting iotex address to address")
					}
					subChainAddr := address.New(cfg.Chain.ID, rootChainAddr.Payload())
					pubKey, err := keypair.DecodePublicKey(rawc.PubKey)
					if err != nil {
						logger.Error().Err(err).Msg("error when convert candidate PublicKey")
					}
					votes, ok := big.NewInt(0).SetString(rawc.TotalVote, 10)
					if !ok {
						logger.Error().Err(err).Msg("error when setting candidate total votes")
					}
					cs = append(cs, &state.Candidate{
						Address:          subChainAddr.IotxAddress(),
						PublicKey:        pubKey,
						Votes:            votes,
						CreationHeight:   uint64(rawc.CreationHeight),
						LastUpdateHeight: uint64(rawc.LastUpdateHeight),
					})
				}
				return cs, nil
			})
			bd = bd.SetRootChainAPI(ops.rootChainAPI)
		}
		cs.scheme, err = bd.Build()
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

// HandleBlockPropose handles a proposed block
func (c *IotxConsensus) HandleBlockPropose(propose *iproto.ProposePb) error {
	return c.scheme.HandleBlockPropose(propose)
}

// HandleEndorse handle an endorse
func (c *IotxConsensus) HandleEndorse(endorse *iproto.EndorsePb) error {
	return c.scheme.HandleEndorse(endorse)
}

// Scheme returns the scheme instance
func (c *IotxConsensus) Scheme() scheme.Scheme {
	return c.scheme
}

// GetAddr returns the iotex address
func GetAddr(cfg *config.Config) *iotxaddress.Address {
	addr, err := cfg.BlockchainAddress()
	if err != nil {
		logger.Panic().Err(err).Msg("Fail to create new consensus")
	}
	pk, err := keypair.DecodePublicKey(cfg.Chain.ProducerPubKey)
	if err != nil {
		logger.Panic().Err(err).Msg("Fail to create new consensus")
	}
	sk, err := keypair.DecodePrivateKey(cfg.Chain.ProducerPrivKey)
	if err != nil {
		logger.Panic().Err(err).Msg("Fail to create new consensus")
	}
	return &iotxaddress.Address{
		PublicKey:  pk,
		PrivateKey: sk,
		RawAddress: addr.IotxAddress(),
	}
}
