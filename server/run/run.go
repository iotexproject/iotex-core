// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package run

import (
	"os"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/rpcservice"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/txpool"
)

// Run starts the iotex node and block on the stop chan.
func Run(cfg *config.Config, stop chan struct{}) {
	// create Blockchain and TxPool instance
	defer os.Remove(cfg.Chain.ChainDBPath)
	bc := blockchain.CreateBlockchain(ta.Addrinfo["miner"].Address, cfg)
	tp := txpool.New(bc)
	defer bc.Close()

	overlay := network.NewOverlay(&cfg.Network)
	pool := delegate.NewConfigBasedPool(&cfg.Delegate)
	bs := blocksync.NewBlockSyncer(cfg, bc, tp, overlay, pool)

	// create dispatcher instance
	dp := dispatcher.NewDispatcher(cfg, bc, tp, bs, pool)
	overlay.AttachDispatcher(dp)
	dp.Start()
	defer dp.Stop()

	if err := overlay.Init(); err != nil {
		glog.Fatal(err)
	}

	if err := overlay.Start(); err != nil {
		glog.Fatal(err)
	}
	defer overlay.Stop()

	if err := pool.Init(); err != nil {
		glog.Fatal(err)
	}

	if err := pool.Start(); err != nil {
		glog.Fatal(err)
	}
	defer pool.Stop()

	if cfg.RPC != (config.RPC{}) {
		bcb := func(msg proto.Message) error {
			return bs.P2P().Broadcast(msg)
		}
		cs := rpcservice.NewChainServer(cfg.RPC, bc, dp, bcb)
		cs.Start()
		defer cs.Stop()
	}

	select {
	case <-stop:
	}
}
