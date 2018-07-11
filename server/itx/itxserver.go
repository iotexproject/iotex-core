// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package itx

import (
	"context"
	"os"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/dispatch"
	"github.com/iotexproject/iotex-core/dispatch/dispatcher"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
)

// Server is the iotex server instance containing all components.
type Server struct {
	bc  blockchain.Blockchain
	ap  actpool.ActPool
	o   *network.Overlay
	dp  dispatcher.Dispatcher
	cfg *config.Config
	cs  consensus.Consensus
}

// NewServer creates a new server
func NewServer(cfg *config.Config) *Server {
	// create Blockchain
	bc := blockchain.NewBlockchain(cfg, blockchain.DefaultStateFactoryOption(), blockchain.BoltDBDaoOption())
	return newServer(cfg, bc)
}

// NewInMemTestServer creates a test server in memory
func NewInMemTestServer(cfg *config.Config) *Server {
	bc := blockchain.NewBlockchain(cfg, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	return newServer(cfg, bc)
}

// Start starts the server
func (s *Server) Start(ctx context.Context) error {
	if err := s.dp.Start(ctx); err != nil {
		logger.Error().Err(err)
		return err
	}
	if err := s.o.Start(ctx); err != nil {
		logger.Error().Err(err)
		return err
	}
	return nil
}

// Stop stops the server
func (s *Server) Stop(ctx context.Context) error {
	s.o.Stop(ctx)
	s.dp.Stop(ctx)
	s.bc.Stop(ctx)
	os.Remove(s.cfg.Chain.ChainDBPath)
	return nil
}

// Bc returns the Blockchain
func (s *Server) Bc() blockchain.Blockchain {
	return s.bc
}

// Ap returns the Action pool
func (s *Server) Ap() actpool.ActPool {
	return s.ap
}

// P2p returns the P2P network
func (s *Server) P2p() *network.Overlay {
	return s.o
}

// Dp returns the Dispatcher
func (s *Server) Dp() dispatcher.Dispatcher {
	return s.dp
}

// Cs returns the consensus instance
func (s *Server) Cs() consensus.Consensus {
	return s.cs
}

func newServer(cfg *config.Config, bc blockchain.Blockchain) *Server {

	// create P2P network and BlockSync
	o := network.NewOverlay(&cfg.Network)
	// Create ActPool
	ap, err := actpool.NewActPool(bc, cfg.ActPool)
	if err != nil {
		logger.Fatal().Err(err).Msg("Fail to create actpool")
	}
	pool := delegate.NewConfigBasedPool(&cfg.Delegate)
	bs, err := blocksync.NewBlockSyncer(cfg, bc, ap, o, pool)
	if err != nil {
		logger.Fatal().Err(err).Msg("Fail to create blockSyncer")
	}
	cs := consensus.NewConsensus(cfg, bc, ap, bs, pool)
	if cs == nil {
		logger.Fatal().Msg("Failed to create Consensus")
	}

	// create dispatcher instance
	dp, err := dispatch.NewDispatcher(cfg, ap, bs, cs)
	if err != nil {
		logger.Fatal().Err(err).Msg("Fail to create dispatcher")
	}
	o.AttachDispatcher(dp)

	return &Server{
		bc:  bc,
		ap:  ap,
		o:   o,
		dp:  dp,
		cfg: cfg,
		cs:  cs,
	}
}
