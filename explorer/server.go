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

	"github.com/coopernurse/barrister-go"
	"github.com/pkg/errors"
	"golang.org/x/net/context"

	"github.com/iotexproject/iotex-core/action/protocol/multichain/mainchain"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/indexservice"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
)

// Server is the container of the explorer service
type Server struct {
	cfg     config.Explorer
	exp     explorer.Explorer
	jrpcSvr barrister.Server
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
	idx *indexservice.Server,
) *Server {
	return &Server{
		cfg: cfg,
		exp: &Service{
			bc:  chain,
			c:   consensus,
			dp:  dispatcher,
			ap:  actPool,
			p2p: p2p,
			cfg: cfg,
			idx: idx,
			gs:  GasStation{bc: chain, cfg: cfg},
		},
	}
}

// SetMainChainProtocol sets the main-chain side multi-chain protocol
func (s *Server) SetMainChainProtocol(p *mainchain.Protocol) {
	svr, ok := s.exp.(*Service)
	if !ok {
		return
	}
	svr.SetMainChainProtocol(p)
}

// Start starts the explorer server
func (s *Server) Start(_ context.Context) error {
	portStr := strconv.Itoa(s.cfg.Port)
	started := make(chan bool)
	go func(started chan bool) {
		idl := barrister.MustParseIdlJson([]byte(explorer.IdlJsonRaw))
		s.jrpcSvr = explorer.NewJSONServer(idl, true, s.exp)
		s.jrpcSvr.AddFilter(logFilter{})
		s.httpSvr = http.Server{Handler: &corsAdaptor{expSvr: s.jrpcSvr}}
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
func (s *Server) Explorer() explorer.Explorer { return s.exp }

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

type corsAdaptor struct {
	expSvr barrister.Server
}

func (h corsAdaptor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	w.Header().Set(
		"Access-Control-Allow-Headers",
		"Content-Type, Access-Control-Allow-Headers, Authorization, X-Requested-With",
	)

	h.expSvr.ServeHTTP(w, r)
}
