// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package itx

import (
	"os"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/common/service"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/dispatch"
	"github.com/iotexproject/iotex-core/dispatch/dispatcher"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/state"
)

// Server is the iotex server instance containing all components.
type Server struct {
	service.Service
	bc  blockchain.Blockchain
	ap  actpool.ActPool
	o   *network.Overlay
	dp  dispatcher.Dispatcher
	cfg config.Config
	sf  state.Factory
}

// NewServer creates a new server
func NewServer(cfg config.Config) *Server {
	// create StateFactory
	sf, err := state.NewFactory(&cfg, state.DefaultTrieOption())
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create statefactory")
		return nil
	}
	// create Blockchain
	bc := blockchain.NewBlockchain(&cfg, blockchain.PrecreatedStateFactoryOption(sf), blockchain.BoltDBDaoOption())
	return newServer(cfg, bc, sf)
}

// NewInMemTestServer creates a test server in memory
func NewInMemTestServer(cfg config.Config) *Server {
	sf, err := state.NewFactory(&cfg, state.InMemTrieOption())
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create statefactory")
		return nil
	}
	bc := blockchain.NewBlockchain(&cfg, blockchain.PrecreatedStateFactoryOption(sf), blockchain.InMemDaoOption())
	return newServer(cfg, bc, sf)
}

// Init initialize the server
func (s *Server) Init() error {
	s.dp.Start()
	if err := s.o.Init(); err != nil {
		logger.Error().Err(err)
		return err
	}
	return nil
}

// Start starts the server
func (s *Server) Start() error {
	if err := s.o.Start(); err != nil {
		logger.Error().Err(err)
		return err
	}
	return nil
}

// Stop stops the server
func (s *Server) Stop() {
	s.o.Stop()
	s.dp.Stop()
	s.bc.Stop()
	os.Remove(s.cfg.Chain.ChainDBPath)
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

// Sf returns the StateFactory
func (s *Server) Sf() state.Factory {
	return s.sf
}

func newServer(cfg config.Config, bc blockchain.Blockchain, sf state.Factory) *Server {
	// create P2P network and BlockSync
	o := network.NewOverlay(&cfg.Network)
	// Create ActPool
	ap, err := actpool.NewActPool(sf, cfg.ActPool)
	if err != nil {
		logger.Fatal().Err(err).Msg("Fail to create actpool")
	}
	pool := delegate.NewConfigBasedPool(&cfg.Delegate)
	bs, err := blocksync.NewBlockSyncer(&cfg, bc, ap, o, pool)
	if err != nil {
		logger.Fatal().Err(err)
	}

	// create dispatcher instance
	dp, err := dispatch.NewDispatcher(&cfg, bc, ap, bs, pool, sf)
	if err != nil {
		logger.Fatal().Err(err)
	}
	o.AttachDispatcher(dp)

	return &Server{
		bc:  bc,
		ap:  ap,
		o:   o,
		dp:  dp,
		cfg: cfg,
		sf:  sf,
	}
}
