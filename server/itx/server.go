// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

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
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/api"
	"github.com/iotexproject/iotex-core/v2/chainservice"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/dispatcher"
	"github.com/iotexproject/iotex-core/v2/p2p"
	"github.com/iotexproject/iotex-core/v2/pkg/ha"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/probe"
	"github.com/iotexproject/iotex-core/v2/pkg/routine"
	"github.com/iotexproject/iotex-core/v2/pkg/util/httputil"
	"github.com/iotexproject/iotex-core/v2/server/itx/nodestats"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// Server is the iotex server instance containing all components.
type Server struct {
	cfg                  config.Config
	rootChainService     *chainservice.ChainService
	chainservices        map[uint32]*chainservice.ChainService
	apiServers           map[uint32]*api.ServerV2
	p2pAgent             p2p.Agent
	dispatcher           dispatcher.Dispatcher
	nodeStats            *nodestats.NodeStats
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
func NewInMemTestServer(cfg config.Config) (*Server, error) { // notest
	return newServer(cfg, true)
}

func newServer(cfg config.Config, testing bool) (*Server, error) {
	// TODO: move to a separate package
	actionDeserializer := (&action.Deserializer{}).SetEvmNetworkID(cfg.Chain.EVMNetworkID)
	// create dispatcher instance
	dispatcher, err := dispatcher.NewDispatcher(cfg.Dispatcher, func(msg proto.Message) (string, error) {
		// TODO: support more types of messages
		switch pb := msg.(type) {
		case *iotextypes.Action:
			act, err := actionDeserializer.ActionToSealedEnvelope(pb)
			if err != nil {
				return "", err
			}
			if err := act.VerifySignature(); err != nil {
				return "", err
			}
			pubkey := act.SrcPubkey()
			if pubkey == nil {
				return "", errors.New("public key is nil")
			}
			return string(pubkey.Bytes()), nil
		default:
			return "", nil
		}
	})
	if err != nil {
		return nil, errors.Wrap(err, "fail to create dispatcher")
	}

	var p2pAgent p2p.Agent
	switch cfg.Consensus.Scheme {
	case config.StandaloneScheme:
		p2pAgent = p2p.NewDummyAgent()
	default:
		p2pAgent = p2p.NewAgent(
			cfg.Network,
			cfg.Chain.ID,
			cfg.Genesis.Hash(),
			dispatcher.ValidateMessage,
			dispatcher.HandleBroadcast,
			dispatcher.HandleTell,
		)
	}
	chains := make(map[uint32]*chainservice.ChainService)
	apiServers := make(map[uint32]*api.ServerV2)
	var cs *chainservice.ChainService
	builder := chainservice.NewBuilder(cfg)
	builder.SetP2PAgent(p2pAgent)
	rpcStats := nodestats.NewAPILocalStats()
	builder.SetRPCStats(rpcStats)
	if testing {
		cs, err = builder.BuildForTest()
	} else {
		cs, err = builder.Build()
	}
	if err != nil {
		return nil, errors.Wrap(err, "fail to create chain service")
	}
	nodeStats := nodestats.NewNodeStats(rpcStats, cs.BlockSync(), p2pAgent)
	apiServer, err := cs.NewAPIServer(cfg.API, cfg.Chain.EnableArchiveMode)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create api server")
	}
	if apiServer != nil {
		apiServers[cs.ChainID()] = apiServer
		if err := cs.Blockchain().AddSubscriber(apiServer); err != nil {
			return nil, errors.Wrap(err, "failed to add api server as subscriber")
		}
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
		apiServers:           apiServers,
		nodeStats:            nodeStats,
		initializedSubChains: map[uint32]bool{},
	}
	// Setup sub-chain starter
	// TODO: sub-chain infra should use main-chain API instead of protocol directly
	return &svr, nil
}

// Start starts the server
func (s *Server) Start(ctx context.Context) error {
	cctx, cancel := context.WithCancel(ctx)
	s.subModuleCancel = cancel
	for id, cs := range s.chainservices {
		if err := cs.Start(cctx); err != nil {
			return errors.Wrap(err, "error when starting blockchain")
		}
		if as, ok := s.apiServers[id]; ok {
			if err := as.Start(cctx); err != nil {
				return errors.Wrapf(err, "failed to start api server for chain %d", id)
			}
		}
	}
	if err := s.p2pAgent.Start(cctx); err != nil {
		return errors.Wrap(err, "error when starting P2P agent")
	}
	if err := s.dispatcher.Start(cctx); err != nil {
		return errors.Wrap(err, "error when starting dispatcher")
	}
	if err := s.nodeStats.Start(cctx); err != nil {
		return errors.Wrap(err, "error when starting node stats")
	}
	return nil
}

// Stop stops the server
func (s *Server) Stop(ctx context.Context) error {
	defer s.subModuleCancel()
	if err := s.nodeStats.Stop(ctx); err != nil {
		return errors.Wrap(err, "error when stopping node stats")
	}
	if err := s.p2pAgent.Stop(ctx); err != nil {
		// notest
		return errors.Wrap(err, "error when stopping P2P agent")
	}
	if err := s.dispatcher.Stop(ctx); err != nil {
		// notest
		return errors.Wrap(err, "error when stopping dispatcher")
	}
	for id, cs := range s.chainservices {
		if as, ok := s.apiServers[id]; ok {
			if err := as.Stop(ctx); err != nil {
				return errors.Wrapf(err, "error when stopping api server")
			}
		}
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
	// TODO: explorer dependency deleted here at #1085, need to revive by migrating to api
	builder := chainservice.NewBuilder(cfg)
	cs, err := builder.SetP2PAgent(s.p2pAgent).BuildForSubChain()
	if err != nil {
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

// Config returns the server's config
func (s *Server) Config() config.Config {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.cfg
}

// P2PAgent returns the P2P agent
func (s *Server) P2PAgent() p2p.Agent {
	return s.p2pAgent
}

// ChainService returns the chainservice hold in Server with given id.
func (s *Server) ChainService(id uint32) *chainservice.ChainService {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.chainservices[id]
}

// APIServer returns the API Server hold in Server with given id.
func (s *Server) APIServer(id uint32) *api.ServerV2 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.apiServers[id]
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
	defer func() {
		if err := svr.Stop(context.Background()); err != nil {
			log.L().Panic("Failed to stop server.", zap.Error(err))
		}
	}()
	if _, isGateway := cfg.Plugins[config.GatewayPlugin]; isGateway && cfg.API.ReadyDuration > 0 {
		// wait for a while to make sure the server is ready
		// The original intention was to ensure that all transactions that were not received during the restart were included in block, thereby avoiding inconsistencies in the state of the API node.
		log.L().Info("Waiting for server to be ready.", zap.Duration("duration", cfg.API.ReadyDuration))
		time.Sleep(cfg.API.ReadyDuration)
	}
	if err := probeSvr.TurnOn(); err != nil {
		log.L().Panic("Failed to turn on probe server.", zap.Error(err))
	}
	log.L().Info("Server is ready.")

	if cfg.System.HeartbeatInterval > 0 {
		task := routine.NewRecurringTask(NewHeartbeatHandler(svr, cfg.Network).Log, cfg.System.HeartbeatInterval)
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
		adminserv = httputil.NewServer(port, mux)
		defer func() {
			if err := adminserv.Shutdown(ctx); err != nil {
				log.L().Error("Error when serving metrics data.", zap.Error(err))
			}
		}()
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
	if err := probeSvr.TurnOff(); err != nil {
		log.L().Panic("Failed to turn off probe server.", zap.Error(err))
	}
}
