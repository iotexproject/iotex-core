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
	"github.com/iotexproject/iotex-core/dispatch"
	"github.com/iotexproject/iotex-core/dispatch/dispatcher"
	"github.com/iotexproject/iotex-core/explorer"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
)

// Server is the iotex server instance containing all components.
type Server struct {
	chain      blockchain.Blockchain
	actPool    actpool.ActPool
	p2p        network.Overlay
	dispatcher dispatcher.Dispatcher
	cfg        *config.Config
	consensus  consensus.Consensus
	explorer   *explorer.Server
}

// NewServer creates a new server
func NewServer(cfg *config.Config) *Server {
	// create Blockchain
	chain := blockchain.NewBlockchain(cfg, blockchain.DefaultStateFactoryOption(), blockchain.BoltDBDaoOption())
	if chain == nil && cfg.Chain.EnableFallBackToFreshDB {
		logger.Warn().Msg("Chain db and trie db are falling back to fresh ones")
		if err := os.Rename(cfg.Chain.ChainDBPath, cfg.Chain.ChainDBPath+".old"); err != nil {
			logger.Error().Err(err).Msg("Failed to rename old chain db")
			return nil
		}
		if err := os.Rename(cfg.Chain.TrieDBPath, cfg.Chain.TrieDBPath+".old"); err != nil {
			logger.Error().Err(err).Msg("Failed to rename old trie db")
			return nil
		}
		chain = blockchain.NewBlockchain(cfg, blockchain.DefaultStateFactoryOption(), blockchain.BoltDBDaoOption())

	}
	return newServer(cfg, chain)
}

// NewInMemTestServer creates a test server in memory
func NewInMemTestServer(cfg *config.Config) *Server {
	chain := blockchain.NewBlockchain(cfg, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	return newServer(cfg, chain)
}

// Start starts the server
func (s *Server) Start(ctx context.Context) error {
	if err := s.dispatcher.Start(ctx); err != nil {
		logger.Panic().Err(err).Msg("error when starting dispatcher")
		return nil
	}
	if err := s.p2p.Start(ctx); err != nil {
		logger.Panic().Err(err).Msg("error when starting P2P networks")
		return nil
	}
	if err := s.explorer.Start(ctx); err != nil {
		logger.Panic().Err(err).Msg("error when starting explorer")
		return nil
	}
	return nil
}

// Stop stops the server
func (s *Server) Stop(ctx context.Context) error {
	if err := s.explorer.Stop(ctx); err != nil {
		logger.Panic().Err(err).Msg("error when stopping explorer")
		return nil
	}
	if err := s.p2p.Stop(ctx); err != nil {
		logger.Panic().Err(err).Msg("error when stopping P2P networks")
		return nil
	}
	if err := s.dispatcher.Stop(ctx); err != nil {
		logger.Panic().Err(err).Msg("error when stopping dispatcher")
		return nil
	}
	if err := s.chain.Stop(ctx); err != nil {
		logger.Panic().Err(err).Msg("error when stopping blockchain")
		return nil
	}
	return nil
}

// Blockchain returns the Blockchain
func (s *Server) Blockchain() blockchain.Blockchain {
	return s.chain
}

// ActionPool returns the Action pool
func (s *Server) ActionPool() actpool.ActPool {
	return s.actPool
}

// P2P returns the P2P network
func (s *Server) P2P() network.Overlay {
	return s.p2p
}

// Dispatcher returns the Dispatcher
func (s *Server) Dispatcher() dispatcher.Dispatcher {
	return s.dispatcher
}

// Consensus returns the consensus instance
func (s *Server) Consensus() consensus.Consensus {
	return s.consensus
}

// Explorer returns the explorer instance
func (s *Server) Explorer() *explorer.Server {
	return s.explorer
}

func newServer(cfg *config.Config, chain blockchain.Blockchain) *Server {
	// create P2P network and BlockSync
	p2p := network.NewOverlay(&cfg.Network)
	// Create ActPool
	actPool, err := actpool.NewActPool(chain, cfg.ActPool)
	if err != nil {
		logger.Fatal().Err(err).Msg("Fail to create actpool")
	}
	bs, err := blocksync.NewBlockSyncer(cfg, chain, actPool, p2p)
	if err != nil {
		logger.Fatal().Err(err).Msg("Fail to create blockSyncer")
	}
	consensus := consensus.NewConsensus(cfg, chain, actPool, p2p)
	if consensus == nil {
		logger.Fatal().Msg("Failed to create Consensus")
	}

	// create dispatcher instance
	dispatcher, err := dispatch.NewDispatcher(cfg, actPool, bs, consensus)
	if err != nil {
		logger.Fatal().Err(err).Msg("Fail to create dispatcher")
	}
	p2p.AttachDispatcher(dispatcher)

	var exp *explorer.Server
	if cfg.Explorer.IsTest || os.Getenv("APP_ENV") == "development" {
		logger.Warn().Msg("Using test server with fake data...")
		exp = explorer.NewTestSever(cfg.Explorer)
	} else {
		exp = explorer.NewServer(cfg.Explorer, chain, consensus, dispatcher, actPool, p2p)
	}

	return &Server{
		cfg:        cfg,
		chain:      chain,
		actPool:    actPool,
		p2p:        p2p,
		dispatcher: dispatcher,
		consensus:  consensus,
		explorer:   exp,
	}
}
