// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package itx

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"runtime"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/multichain/mainchain"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/chainservice"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/ha"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/probe"
	"github.com/iotexproject/iotex-core/pkg/routine"
	"github.com/iotexproject/iotex-core/pkg/util/httputil"
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
	subModuleCancel      context.CancelFunc
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
	p2pAgent := p2p.NewAgent(cfg, dispatcher.HandleBroadcast, dispatcher.HandleTell)
	chains := make(map[uint32]*chainservice.ChainService)
	var cs *chainservice.ChainService
	var opts []chainservice.Option
	if testing {
		opts = []chainservice.Option{
			chainservice.WithTesting(),
		}
	}
	cs, err = chainservice.New(cfg, p2pAgent, dispatcher, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "fail to create chain service")
	}

	// Add action validators
	cs.ActionPool().
		AddActionEnvelopeValidators(
			protocol.NewGenericValidator(cs.Blockchain(), cfg.Genesis.ActionGasLimit),
		)
	cs.Blockchain().Validator().
		AddActionEnvelopeValidators(
			protocol.NewGenericValidator(cs.Blockchain(), cfg.Genesis.ActionGasLimit),
		)
	// Install protocols
	if err := registerDefaultProtocols(cs, cfg); err != nil {
		return nil, err
	}
	mainChainProtocol := mainchain.NewProtocol(cs.Blockchain())
	if err := cs.RegisterProtocol(mainchain.ProtocolID, mainChainProtocol); err != nil {
		return nil, err
	}
	// TODO: explorer dependency deleted here at #1085, need to revive by migrating to api
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
	cctx, cancel := context.WithCancel(context.Background())
	s.subModuleCancel = cancel
	if err := s.p2pAgent.Start(cctx); err != nil {
		return errors.Wrap(err, "error when starting P2P agent")
	}
	if err := s.rootChainService.Blockchain().AddSubscriber(s); err != nil {
		return errors.Wrap(err, "error when starting sub-chain starter")
	}
	for _, cs := range s.chainservices {
		if err := cs.Start(cctx); err != nil {
			return errors.Wrap(err, "error when starting blockchain")
		}
	}
	if err := s.dispatcher.Start(cctx); err != nil {
		return errors.Wrap(err, "error when starting dispatcher")
	}

	return nil
}

// Stop stops the server
func (s *Server) Stop(ctx context.Context) error {
	defer s.subModuleCancel()
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
func (s *Server) NewSubChainService(cfg config.Config, opts ...chainservice.Option) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.newSubChainService(cfg, opts...)
}

func (s *Server) newSubChainService(cfg config.Config, opts ...chainservice.Option) error {
	// TODO: explorer dependency deleted here at #1085, need to revive by migrating to api
	cs, err := chainservice.New(cfg, s.p2pAgent, s.dispatcher, opts...)
	if err != nil {
		return err
	}
	cs.ActionPool().
		AddActionEnvelopeValidators(
			protocol.NewGenericValidator(cs.Blockchain(), cfg.Genesis.ActionGasLimit),
		)
	cs.Blockchain().Validator().
		AddActionEnvelopeValidators(
			protocol.NewGenericValidator(cs.Blockchain(), cfg.Genesis.ActionGasLimit),
		)
	if err := registerDefaultProtocols(cs, cfg); err != nil {
		return err
	}
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

	var adminserv http.Server
	if cfg.System.HTTPAdminPort > 0 {
		mux := http.NewServeMux()
		log.RegisterLevelConfigMux(mux)
		haCtl := ha.New(svr.rootChainService.Consensus())
		mux.Handle("/ha", http.HandlerFunc(haCtl.Handle))
		mux.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
		mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
		mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		mux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		mux.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))

		port := fmt.Sprintf(":%d", cfg.System.HTTPAdminPort)
		adminserv = httputil.Server(port, mux)
		go func() {
			runtime.SetMutexProfileFraction(1)
			runtime.SetBlockProfileRate(1)
			ln, err := httputil.LimitListener(adminserv.Addr)
			if err != nil {
				log.L().Error("Error when listen to profiling port.", zap.Error(err))
				return
			}
			if err := adminserv.Serve(ln); err != nil {
				log.L().Error("Error when serving performance profiling data.", zap.Error(err))
			}
		}()
	}

	<-ctx.Done()
	probeSvr.NotReady()
	if err := adminserv.Shutdown(ctx); err != nil {
		log.L().Error("Error when serving metrics data.", zap.Error(err))
	}
	if err := svr.Stop(ctx); err != nil {
		log.L().Panic("Failed to stop server.", zap.Error(err))
	}
}

func registerDefaultProtocols(cs *chainservice.ChainService, cfg config.Config) (err error) {
	genesisConfig := cfg.Genesis
	accountProtocol := account.NewProtocol(genesisConfig.PacificBlockHeight)
	if err = cs.RegisterProtocol(account.ProtocolID, accountProtocol); err != nil {
		return
	}
	rolldposProtocol := cs.RollDPoSProtocol()
	if err = cs.RegisterProtocol(rolldpos.ProtocolID, rolldposProtocol); err != nil {
		return
	}
	if cfg.Consensus.Scheme == config.RollDPoSScheme && genesisConfig.EnableGravityChainVoting {
		electionCommittee := cs.ElectionCommittee()
		gravityChainStartHeight := genesisConfig.GravityChainStartHeight
		var pollProtocol poll.Protocol
		if genesisConfig.GravityChainStartHeight != 0 && electionCommittee != nil {
			if pollProtocol, err = poll.NewGovernanceChainCommitteeProtocol(
				cs.Blockchain(),
				electionCommittee,
				gravityChainStartHeight,
				func(height uint64) (time.Time, error) {
					header, err := cs.Blockchain().BlockHeaderByHeight(height)
					if err != nil {
						return time.Now(), errors.Wrapf(
							err, "error when getting the block at height: %d",
							height,
						)
					}
					return header.Timestamp(), nil
				},
				rolldposProtocol.GetEpochHeight,
				rolldposProtocol.GetEpochNum,
				genesisConfig.NumCandidateDelegates,
				genesisConfig.NumDelegates,
				cfg.Chain.PollInitialCandidatesInterval,
			); err != nil {
				return
			}
		} else {
			delegates := genesisConfig.Delegates
			if uint64(len(delegates)) < genesisConfig.NumDelegates {
				return errors.New("invalid delegate address in genesis block")
			}
			pollProtocol = poll.NewLifeLongDelegatesProtocol(delegates)
		}
		if err = cs.RegisterProtocol(poll.ProtocolID, pollProtocol); err != nil {
			return
		}
	}
	executionProtocol := execution.NewProtocol(cs.Blockchain(), genesisConfig.PacificBlockHeight)
	if err = cs.RegisterProtocol(execution.ProtocolID, executionProtocol); err != nil {
		return
	}
	rewardingProtocol := rewarding.NewProtocol(cs.Blockchain(), rolldposProtocol)
	return cs.RegisterProtocol(rewarding.ProtocolID, rewardingProtocol)
}
