// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package itx

import (
	"context"
	"fmt"
	"net/http"
	"runtime"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/multichain/mainchain"
	"github.com/iotexproject/iotex-core/action/protocol/multichain/subchain"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/chainservice"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/pkg/routine"
)

// Server is the iotex server instance containing all components.
type Server struct {
	cfg                  config.Config
	rootChainService     *chainservice.ChainService
	chainservices        map[uint32]*chainservice.ChainService
	p2p                  network.Overlay
	dispatcher           dispatcher.Dispatcher
	mainChainProtocol    *mainchain.Protocol
	initializedSubChains map[uint32]bool
}

// NewServer creates a new server
// TODO clean up config, make root config contains network, dispatch and chainservice
func NewServer(cfg config.Config) (*Server, error) {
	return newServer(cfg, false)
}

// NewInMemTestServer creates a test server in memory
func NewInMemTestServer(cfg config.Config) (*Server, error) {
	return newServer(cfg, true)
}

func newServer(cfg config.Config, testing bool) (*Server, error) {
	// create P2P network and BlockSync
	p2p := network.NewOverlay(cfg.Network)

	// create dispatcher instance
	dispatcher, err := dispatcher.NewDispatcher(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "fail to create dispatcher")
	}
	p2p.AttachDispatcher(dispatcher)

	chains := make(map[uint32]*chainservice.ChainService)

	var cs *chainservice.ChainService

	var opts []chainservice.Option
	if testing {
		opts = []chainservice.Option{chainservice.WithTesting()}
	}
	cs, err = chainservice.New(cfg, p2p, dispatcher, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "fail to create chain service")
	}

	// Add action validators
	cs.ActionPool().
		AddActionValidators(
			actpool.NewGenericValidator(cs.Blockchain()),
		)
	// Install protocols
	mainChainProtocol := mainchain.NewProtocol(cs.Blockchain())
	accountProtocol := account.NewProtocol()
	voteProtocol := vote.NewProtocol(cs.Blockchain())
	executionProtocol := execution.NewProtocol(cs.Blockchain())
	cs.AddProtocols(mainChainProtocol, accountProtocol, voteProtocol, executionProtocol)
	if cs.Explorer() != nil {
		cs.Explorer().SetMainChainProtocol(mainChainProtocol)
	}

	chains[cs.ChainID()] = cs
	dispatcher.AddSubscriber(cs.ChainID(), cs)
	svr := Server{
		cfg:                  cfg,
		p2p:                  p2p,
		dispatcher:           dispatcher,
		rootChainService:     cs,
		chainservices:        chains,
		mainChainProtocol:    mainChainProtocol,
		initializedSubChains: map[uint32]bool{},
	}
	// Setup sub-chain starter
	// TODO: sub-chain infra should use main-chain API instead of protocol directly
	return &svr, nil
}

// Start starts the server
func (s *Server) Start(ctx context.Context) error {
	if err := s.rootChainService.Blockchain().AddSubscriber(s); err != nil {
		return errors.Wrap(err, "error when starting sub-chain starter")
	}
	for _, cs := range s.chainservices {
		if err := cs.Start(ctx); err != nil {
			return errors.Wrap(err, "error when stopping blockchain")
		}
	}
	if err := s.dispatcher.Start(ctx); err != nil {
		return errors.Wrap(err, "error when starting dispatcher")
	}
	if err := s.p2p.Start(ctx); err != nil {
		return errors.Wrap(err, "error when starting P2P networks")
	}

	return nil
}

// Stop stops the server
func (s *Server) Stop(ctx context.Context) error {
	if err := s.rootChainService.Blockchain().RemoveSubscriber(s); err != nil {
		return errors.Wrap(err, "error when unsubscribing root chain block creation")
	}
	if err := s.p2p.Stop(ctx); err != nil {
		return errors.Wrap(err, "error when stopping P2P networks")
	}
	if err := s.dispatcher.Stop(ctx); err != nil {
		return errors.Wrap(err, "error when stopping dispatcher")
	}
	for _, cs := range s.chainservices {
		if err := cs.Stop(ctx); err != nil {
			return errors.Wrap(err, "error when stopping blockchain")
		}
	}
	return nil
}

// NewSubChainService creates a new chain service in this server.
func (s *Server) NewSubChainService(cfg config.Config) error {
	return s.newSubChainService(cfg)
}

func (s *Server) newSubChainService(cfg config.Config) error {
	var mainChainAPI explorer.Explorer
	if s.rootChainService.Explorer() != nil {
		mainChainAPI = s.rootChainService.Explorer().Explorer()
	}
	opts := []chainservice.Option{chainservice.WithRootChainAPI(mainChainAPI)}
	cs, err := chainservice.New(cfg, s.p2p, s.dispatcher, opts...)
	if err != nil {
		return err
	}
	cs.ActionPool().
		AddActionValidators(
			actpool.NewGenericValidator(cs.Blockchain()),
		)
	subChainProtocol := subchain.NewProtocol(cs.Blockchain(), mainChainAPI)
	accountProtocol := account.NewProtocol()
	voteProtocol := vote.NewProtocol(cs.Blockchain())
	executionProtocol := execution.NewProtocol(cs.Blockchain())
	cs.AddProtocols(subChainProtocol, accountProtocol, voteProtocol, executionProtocol)
	s.chainservices[cs.ChainID()] = cs
	s.dispatcher.AddSubscriber(cs.ChainID(), cs)
	return nil
}

// NewTestingChainService creates a new testing chain service in this server.
func (s *Server) NewTestingChainService(cfg config.Config) error {
	var mainChainAPI explorer.Explorer
	if s.rootChainService.Explorer() != nil {
		mainChainAPI = s.rootChainService.Explorer().Explorer()
	}
	opts := []chainservice.Option{
		chainservice.WithTesting(),
		chainservice.WithRootChainAPI(mainChainAPI),
	}
	cs, err := chainservice.New(cfg, s.p2p, s.dispatcher, opts...)
	if err != nil {
		return err
	}
	cs.ActionPool().
		AddActionValidators(
			actpool.NewGenericValidator(cs.Blockchain()),
		)
	subChainProtocol := subchain.NewProtocol(cs.Blockchain(), mainChainAPI)
	accountProtocol := account.NewProtocol()
	voteProtocol := vote.NewProtocol(cs.Blockchain())
	executionProtocol := execution.NewProtocol(cs.Blockchain())
	cs.AddProtocols(subChainProtocol, accountProtocol, voteProtocol, executionProtocol)
	s.chainservices[cs.ChainID()] = cs
	s.dispatcher.AddSubscriber(cs.ChainID(), cs)
	return nil
}

// StopChainService stops the chain service run in the server.
func (s *Server) StopChainService(ctx context.Context, id uint32) error {
	c, ok := s.chainservices[id]
	if !ok {
		return errors.New("Chain ID does not match any existing chains")
	}
	return c.Stop(ctx)
}

// P2P returns the P2P network
func (s *Server) P2P() network.Overlay {
	return s.p2p
}

// ChainService returns the chainservice hold in Server with given id.
func (s *Server) ChainService(id uint32) *chainservice.ChainService { return s.chainservices[id] }

// Dispatcher returns the Dispatcher
func (s *Server) Dispatcher() dispatcher.Dispatcher {
	return s.dispatcher
}

// StartServer starts a node server
func StartServer(svr *Server, cfg config.Config) {
	ctx := context.Background()
	if err := svr.Start(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start server.")
		return
	}
	defer func() {
		if err := svr.Stop(ctx); err != nil {
			logger.Panic().Err(err).Msg("Failed to stop server.")
		}
	}()

	if cfg.System.HeartbeatInterval > 0 {
		task := routine.NewRecurringTask(NewHeartbeatHandler(svr).Log, cfg.System.HeartbeatInterval)
		if err := task.Start(ctx); err != nil {
			logger.Panic().Err(err).Msg("Failed to start heartbeat routine.")
		}
		defer func() {
			if err := task.Stop(ctx); err != nil {
				logger.Panic().Err(err).Msg("Failed to stop heartbeat routine.")
			}
		}()
	}

	if cfg.System.HTTPProfilingPort > 0 {
		go func() {
			runtime.SetMutexProfileFraction(1)
			runtime.SetBlockProfileRate(1)
			if err := http.ListenAndServe(
				fmt.Sprintf(":%d", cfg.System.HTTPProfilingPort),
				nil,
			); err != nil {
				logger.Error().Err(err).Msg("error when serving performance profiling data")
			}
		}()
	}

	if cfg.System.HTTPMetricsPort > 0 {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		port := fmt.Sprintf(":%d", cfg.System.HTTPMetricsPort)
		go func() {
			if err := http.ListenAndServe(port, mux); err != nil {
				logger.Error().Err(err).Msg("error when serving performance profiling data")
			}
		}()
	}
	select {}
}
