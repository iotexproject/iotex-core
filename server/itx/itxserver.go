// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package itx

import (
	"os"

	"github.com/golang/glog"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blocksync"
	cm "github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/common/service"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/network"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/txpool"
)

// Server is the iotex server instance containing all components.
type Server struct {
	service.Service
	bc  blockchain.IBlockchain
	o   *network.Overlay
	dp  cm.Dispatcher
	cfg config.Config
}

// NewServer creates a new server
func NewServer(cfg config.Config) Server {
	bc := blockchain.CreateBlockchain(ta.Addrinfo["miner"].Address, &cfg)
	tp := txpool.New(bc)

	// server use first BootstrapNodes addr
	o := network.NewOverlay(&cfg.Network)
	pool := delegate.NewConfigBasedPool(&cfg.Delegate)
	bs := blocksync.NewBlockSyncer(&cfg, bc, tp, o, pool)

	// create dispatcher instance
	dp := dispatcher.NewDispatcher(&cfg, bc, tp, bs, pool)
	o.AttachDispatcher(dp)

	return Server{
		bc:  bc,
		o:   o,
		dp:  dp,
		cfg: cfg,
	}
}

// Init initialize the server
func (s *Server) Init() {
	s.dp.Start()
	if err := s.o.Init(); err != nil {
		glog.Fatal(err)
	}
}

// Start starts the server
func (s *Server) Start() {
	if err := s.o.Start(); err != nil {
		glog.Fatal(err)
	}
}

// Stop stops the server
func (s *Server) Stop() {
	s.o.Stop()
	s.dp.Stop()
	s.bc.Close()
	os.Remove(s.cfg.Chain.ChainDBPath)
}
