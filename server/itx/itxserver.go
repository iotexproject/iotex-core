// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package itx

import (
	"os"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/common/service"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/dispatch"
	"github.com/iotexproject/iotex-core/dispatch/dispatcher"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/statefactory"
	"github.com/iotexproject/iotex-core/txpool"
)

// Server is the iotex server instance containing all components.
type Server struct {
	service.Service
	bc  blockchain.Blockchain
	tp  txpool.TxPool
	ap  txpool.ActPool
	o   *network.Overlay
	dp  dispatcher.Dispatcher
	cfg config.Config
}

// NewServer creates a new server
func NewServer(cfg config.Config) *Server {
	// create StateFactory
	sf, err := statefactory.NewStateFactoryTrieDB(cfg.Chain.TrieDBPath)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create statefactory")
		return nil
	}
	// create Blockchain
	bc := blockchain.CreateBlockchain(&cfg, sf)
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

// Tp returns the TxPool
func (s *Server) Tp() txpool.TxPool {
	return s.tp
}

// P2p returns the P2P network
func (s *Server) P2p() *network.Overlay {
	return s.o
}

// Dp returns the Dispatcher
func (s *Server) Dp() dispatcher.Dispatcher {
	return s.dp
}

func newServer(cfg config.Config, bc blockchain.Blockchain, sf statefactory.StateFactory) *Server {
	// create TxPool
	tp := txpool.NewTxPool(bc)
	// create P2P network and BlockSync
	o := network.NewOverlay(&cfg.Network)
	// Create ActPool
	ap := txpool.NewActPool(sf, o)
	pool := delegate.NewConfigBasedPool(&cfg.Delegate)
	bs := blocksync.NewBlockSyncer(&cfg, bc, tp, ap, o, pool)
	// create dispatcher instance
	dp := dispatch.NewDispatcher(&cfg, bc, tp, ap, bs, pool, sf)
	o.AttachDispatcher(dp)

	return &Server{
		bc:  bc,
		tp:  tp,
		ap:  ap,
		o:   o,
		dp:  dp,
		cfg: cfg,
	}
}
