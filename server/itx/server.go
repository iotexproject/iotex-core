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
	"sync"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/multichain/mainchain"
	"github.com/iotexproject/iotex-core/action/protocol/multichain/subchain"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/chainservice"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/probe"
	"github.com/iotexproject/iotex-core/pkg/routine"
)

// Server is the iotex server instance containing all components.
type Server struct {
	cfg                  config.Config
	rootChainService     *chainservice.ChainService
	chainservices        map[uint32]*chainservice.ChainService
	p2pAgent             *p2p.Agent
	dispatcher           dispatcher.Dispatcher
	mainChainProtocol    *mainchain.Protocol
	initializedSubChains map[uint32]bool
	mutex                sync.RWMutex
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
	// create dispatcher instance
	dispatcher, err := dispatcher.NewDispatcher(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "fail to create dispatcher")
	}
	p2pAgent := p2p.NewAgent(cfg.Network, dispatcher.HandleBroadcast, dispatcher.HandleTell)
	chains := make(map[uint32]*chainservice.ChainService)
	var cs *chainservice.ChainService
	var opts []chainservice.Option
	if testing {
		opts = []chainservice.Option{chainservice.WithTesting()}
	}
	cs, err = chainservice.New(cfg, p2pAgent, dispatcher, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "fail to create chain service")
	}

	// Add action validators
	cs.ActionPool().
		AddActionEnvelopeValidators(
			protocol.NewGenericValidator(cs.Blockchain()),
		)
	cs.Blockchain().Validator().
		AddActionEnvelopeValidators(
			protocol.NewGenericValidator(cs.Blockchain()),
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
		p2pAgent:             p2pAgent,
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
	if err := s.p2pAgent.Start(ctx); err != nil {
		return errors.Wrap(err, "error when starting P2P agent")
	}
	if err := s.rootChainService.Blockchain().AddSubscriber(s); err != nil {
		return errors.Wrap(err, "error when starting sub-chain starter")
	}
	for _, cs := range s.chainservices {
		if err := cs.Start(ctx); err != nil {
			return errors.Wrap(err, "error when starting blockchain")
		}
	}
	if err := s.dispatcher.Start(ctx); err != nil {
		return errors.Wrap(err, "error when starting dispatcher")
	}

	return nil
}

// Stop stops the server
func (s *Server) Stop(ctx context.Context) error {
	if err := s.p2pAgent.Stop(ctx); err != nil {
		return errors.Wrap(err, "error when stopping P2P agent")
	}
	if err := s.dispatcher.Stop(ctx); err != nil {
		return errors.Wrap(err, "error when stopping dispatcher")
	}
	if err := s.rootChainService.Blockchain().RemoveSubscriber(s); err != nil {
		return errors.Wrap(err, "error when unsubscribing root chain block creation")
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
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.newSubChainService(cfg)
}

func (s *Server) newSubChainService(cfg config.Config) error {
	var mainChainAPI explorer.Explorer
	if s.rootChainService.Explorer() != nil {
		mainChainAPI = s.rootChainService.Explorer().Explorer()
	}
	opts := []chainservice.Option{chainservice.WithRootChainAPI(mainChainAPI)}
	cs, err := chainservice.New(cfg, s.p2pAgent, s.dispatcher, opts...)
	if err != nil {
		return err
	}
	cs.ActionPool().
		AddActionEnvelopeValidators(
			protocol.NewGenericValidator(cs.Blockchain()),
		)
	cs.Blockchain().Validator().
		AddActionEnvelopeValidators(
			protocol.NewGenericValidator(cs.Blockchain()),
		)
	subChainProtocol := subchain.NewProtocol(cs.Blockchain(), mainChainAPI)
	accountProtocol := account.NewProtocol()
	voteProtocol := vote.NewProtocol(cs.Blockchain())
	executionProtocol := execution.NewProtocol(cs.Blockchain())
	cs.AddProtocols(subChainProtocol, accountProtocol, voteProtocol, executionProtocol)
	s.chainservices[cs.ChainID()] = cs
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
	cs, err := chainservice.New(cfg, s.p2pAgent, s.dispatcher, opts...)
	if err != nil {
		return err
	}
	cs.ActionPool().
		AddActionEnvelopeValidators(
			protocol.NewGenericValidator(cs.Blockchain()),
		)
	cs.Blockchain().Validator().
		AddActionEnvelopeValidators(
			protocol.NewGenericValidator(cs.Blockchain()),
		)
	subChainProtocol := subchain.NewProtocol(cs.Blockchain(), mainChainAPI)
	accountProtocol := account.NewProtocol()
	voteProtocol := vote.NewProtocol(cs.Blockchain())
	executionProtocol := execution.NewProtocol(cs.Blockchain())
	cs.AddProtocols(subChainProtocol, accountProtocol, voteProtocol, executionProtocol)
	s.chainservices[cs.ChainID()] = cs
	return nil
}

// StopChainService stops the chain service run in the server.
func (s *Server) StopChainService(ctx context.Context, id uint32) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	c, ok := s.chainservices[id]
	if !ok {
		return errors.New("Chain ID does not match any existing chains")
	}
	return c.Stop(ctx)
}

// P2PAgent returns the P2P agent
func (s *Server) P2PAgent() *p2p.Agent {
	return s.p2pAgent
}

// ChainService returns the chainservice hold in Server with given id.
func (s *Server) ChainService(id uint32) *chainservice.ChainService {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.chainservices[id]
}

// Dispatcher returns the Dispatcher
func (s *Server) Dispatcher() dispatcher.Dispatcher {
	return s.dispatcher
}

// StartServer starts a node server
func StartServer(ctx context.Context, svr *Server, probeSvr *probe.Server, cfg config.Config) {
	if err := svr.Start(ctx); err != nil {
		log.L().Fatal("Failed to start server.", zap.Error(err))
		return
	}
	probeSvr.Ready()

	if cfg.System.HeartbeatInterval > 0 {
		task := routine.NewRecurringTask(NewHeartbeatHandler(svr).Log, cfg.System.HeartbeatInterval)
		if err := task.Start(ctx); err != nil {
			log.L().Panic("Failed to start heartbeat routine.", zap.Error(err))
		}
		defer func() {
			if err := task.Stop(ctx); err != nil {
				log.L().Panic("Failed to stop heartbeat routine.", zap.Error(err))
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
				log.L().Error("Error when serving performance profiling data.", zap.Error(err))
			}
		}()
	}

	var mserv http.Server
	if cfg.System.HTTPMetricsPort > 0 {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		log.RegisterLevelConfigMux(mux)
		port := fmt.Sprintf(":%d", cfg.System.HTTPMetricsPort)
		mserv = http.Server{
			Addr:    port,
			Handler: mux,
		}
		go func() {
			if err := mserv.ListenAndServe(); err != nil {
				log.L().Error("Error when serving metrics data.", zap.Error(err))
			}
		}()
	}

	<-ctx.Done()
	probeSvr.NotReady()
	if err := mserv.Shutdown(ctx); err != nil {
		log.L().Error("Error when serving metrics data.", zap.Error(err))
	}
	if err := svr.Stop(ctx); err != nil {
		log.L().Panic("Failed to stop server.", zap.Error(err))
	}
}
