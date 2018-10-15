// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package explorer

import (
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/coopernurse/barrister-go"
	"github.com/pkg/errors"
	"golang.org/x/net/context"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/explorer/idl/web3api"
)

// Server is the container of the explorer service
type Server struct {
	cfg config.Explorer

	expSvc explorer.Explorer
	expSvr barrister.Server

	web3Svc web3api.Web3API
	web3Svr barrister.Server

	httpSvr http.Server
	port    int
}

// NewServer instantiates an explorer server
func NewServer(
	cfg config.Explorer,
	chain blockchain.Blockchain,
	consensus consensus.Consensus,
	dispatcher dispatcher.Dispatcher,
	actPool actpool.ActPool,
	p2p network.Overlay,
) *Server {
	return &Server{
		cfg: cfg,
		expSvc: &Service{
			bc:  chain,
			c:   consensus,
			dp:  dispatcher,
			ap:  actPool,
			p2p: p2p,
			cfg: cfg,
		},
		web3Svc: &PublicWeb3API{
			bc: chain,
		},
	}
}

// NewTestSever instantiates an explorer server with mock web3Svr
func NewTestSever(cfg config.Explorer) *Server {
	return &Server{
		cfg:    cfg,
		expSvc: &MockExplorer{},
	}
}

// Start starts the explorer server
func (s *Server) Start(_ context.Context) error {
	portStr := strconv.Itoa(s.cfg.Port)
	started := make(chan bool)
	go func(started chan bool) {
		// explorer
		expIdl := barrister.MustParseIdlJson([]byte(explorer.IdlJsonRaw))
		s.expSvr = explorer.NewJSONServer(expIdl, true, s.expSvc)
		s.expSvr.AddFilter(logFilter{})

		// web3
		web3Idl := barrister.MustParseIdlJson([]byte(web3api.IdlJsonRaw))
		s.web3Svr = web3api.NewJSONServer(web3Idl, true, s.web3Svc)
		s.web3Svr.AddFilter(logFilter{})

		s.httpSvr = http.Server{Handler: adapterHandler{web3Svr: s.web3Svr, expSvr: s.expSvr}}
		listener, err := net.Listen("tcp", ":"+portStr)
		if err != nil {
			logger.Panic().Err(err).Msg("error when creating network listener")
		}
		logger.Info().Msgf("Starting Explorer JSON-RPC server on %s", listener.Addr().String())
		_, port, err := net.SplitHostPort(listener.Addr().String())
		if err != nil {
			logger.Panic().Err(err).Msgf("error when spliting addr %s", listener.Addr().String())
		}
		s.port, err = strconv.Atoi(port)
		if err != nil {
			logger.Panic().Err(err).Msgf("error when converting port %s to int", port)
		}
		started <- true
		if err := s.httpSvr.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Panic().Err(err).Msg("error when serving JSON-RPC requests")
		}
	}(started)
	<-started
	return nil
}

// Stop stops the explorer server
func (s *Server) Stop(ctx context.Context) error {
	if err := s.httpSvr.Shutdown(ctx); err != nil {
		return errors.Wrap(err, "error when shutting down explorer http server")
	}
	return nil
}

// Port returns the actually binding port
func (s *Server) Port() int {
	return s.port
}

// Explorer returns explorer interface.
func (s *Server) Explorer() explorer.Explorer { return s.expSvc }

// logFilter example of Filter implementation
type logFilter struct{}

// PreInvoke implement empty preinvoke
func (f logFilter) PreInvoke(r *barrister.RequestResponse) bool {
	logger.Debug().Msgf("logFilter: PreInvoke of method: %s", r.Method)
	return true
}

// PostInvoke implement empty postinvoke
func (f logFilter) PostInvoke(r *barrister.RequestResponse) bool {
	logger.Debug().Msgf("logFilter: PostInvoke of method: %s", r.Method)
	return true
}

type adapterHandler struct {
	web3Svr barrister.Server
	expSvr  barrister.Server
}

func (h adapterHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Access-Control-Allow-Headers, Authorization, X-Requested-With")

	if strings.HasPrefix(r.URL.Path, "/web3") {
		h.web3Svr.ServeHTTP(w, r)
		return
	}

	h.expSvr.ServeHTTP(w, r)
}
