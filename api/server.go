// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"net"
	"net/http"
	"strconv"

	"github.com/coopernurse/barrister-go"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/api/idl/api"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/indexservice"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// Config represents the config to setup api
type Config struct {
	broadcastHandler   BroadcastOutbound
	neighborsHandler   Neighbors
	networkInfoHandler NetworkInfo
}

// Option is the option to override the api config
type Option func(cfg *Config) error

// WithBroadcastOutbound is the option to broadcast msg outbound
func WithBroadcastOutbound(broadcastHandler BroadcastOutbound) Option {
	return func(cfg *Config) error {
		cfg.broadcastHandler = broadcastHandler
		return nil
	}
}

// WithNeighbors is the option to set the neighbors callback
func WithNeighbors(neighborsHandler Neighbors) Option {
	return func(cfg *Config) error {
		cfg.neighborsHandler = neighborsHandler
		return nil
	}
}

// WithNetworkInfo is the option to set the network information handler.
func WithNetworkInfo(selfHandler NetworkInfo) Option {
	return func(cfg *Config) error {
		cfg.networkInfoHandler = selfHandler
		return nil
	}
}

// Server is the container of the api service
type Server struct {
	cfg     config.API
	api     api.API
	jrpcSvr barrister.Server
	httpSvr http.Server
	port    int
}

// NewServer instantiates an api server
func NewServer(
	cfg config.API,
	chain blockchain.Blockchain,
	dispatcher dispatcher.Dispatcher,
	actPool actpool.ActPool,
	idx *indexservice.Server,
	opts ...Option,
) (*Server, error) {
	apiCfg := Config{}
	for _, opt := range opts {
		if err := opt(&apiCfg); err != nil {
			return nil, err
		}
	}
	return &Server{
		cfg: cfg,
		api: &Service{
			bc:                 chain,
			dp:                 dispatcher,
			ap:                 actPool,
			broadcastHandler:   apiCfg.broadcastHandler,
			neighborsHandler:   apiCfg.neighborsHandler,
			networkInfoHandler: apiCfg.networkInfoHandler,
			cfg:                cfg,
			idx:                idx,
			gs:                 GasStation{bc: chain, cfg: cfg},
		},
	}, nil
}

// Start starts the api server
func (s *Server) Start(_ context.Context) error {
	portStr := strconv.Itoa(s.cfg.Port)
	started := make(chan bool)
	go func(started chan bool) {
		idl := barrister.MustParseIdlJson([]byte(api.IdlJsonRaw))
		s.jrpcSvr = api.NewJSONServer(idl, true, s.api)
		s.jrpcSvr.AddFilter(logFilter{})
		s.httpSvr = http.Server{Handler: &corsAdaptor{apiSvr: s.jrpcSvr}}
		listener, err := net.Listen("tcp", ":"+portStr)
		if err != nil {
			log.L().Panic("Error when creating network listener", zap.Error(err))
		}
		log.S().Infof("Starting API JSON-RPC server on %s", listener.Addr().String())
		_, port, err := net.SplitHostPort(listener.Addr().String())
		if err != nil {
			log.L().Panic("Error when spliting address.",
				zap.String("address", listener.Addr().String()),
				zap.Error(err))
		}
		s.port, err = strconv.Atoi(port)
		if err != nil {
			log.L().Panic("Error when converting port to int.", zap.String("port", port), zap.Error(err))
		}
		started <- true
		if err := s.httpSvr.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.L().Panic("Error when serving JSON-RPC requests.", zap.Error(err))
		}
	}(started)
	<-started
	return nil
}

// Stop stops the api server
func (s *Server) Stop(ctx context.Context) error {
	if err := s.httpSvr.Shutdown(ctx); err != nil {
		return errors.Wrap(err, "error when shutting down api http server")
	}
	return nil
}

// Port returns the actually binding port
func (s *Server) Port() int {
	return s.port
}

// API returns api interface.
func (s *Server) API() api.API { return s.api }

// logFilter example of Filter implementation
type logFilter struct{}

// PreInvoke implement empty preinvoke
func (f logFilter) PreInvoke(r *barrister.RequestResponse) bool {
	log.S().Debugf("logFilter: PreInvoke of method: %s", r.Method)
	return true
}

// PostInvoke implement empty postinvoke
func (f logFilter) PostInvoke(r *barrister.RequestResponse) bool {
	log.S().Debugf("logFilter: PostInvoke of method: %s", r.Method)
	return true
}

type corsAdaptor struct {
	apiSvr barrister.Server
}

func (h corsAdaptor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	w.Header().Set(
		"Access-Control-Allow-Headers",
		"Content-Type, Access-Control-Allow-Headers, Authorization, X-Requested-With",
	)

	h.apiSvr.ServeHTTP(w, r)
}
