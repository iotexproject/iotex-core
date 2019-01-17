// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// Usage:
//   make build
//   ./bin/server -config-file=./config.yaml
//

package main

import (
	"flag"
	"fmt"
	glog "log"
	"os"

	_ "net/http/pprof"

	_ "go.uber.org/automaxprocs"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/server/itx"
)

// recoveryHeight is the blockchain height being recovered to
var recoveryHeight int

func init() {
	flag.IntVar(&recoveryHeight, "recovery-height", 0, "Recovery height")
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr,
			"usage: server -config-path=[string] -recovery-height=[int]\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()
}

func main() {
	cfg, err := config.New()
	if err != nil {
		glog.Fatalln("Failed to new config.", zap.Error(err))
	}

	initLogger(cfg)

	// create and start the node
	svr, err := itx.NewServer(cfg)
	if err != nil {
		log.L().Fatal("Failed to create server.", zap.Error(err))
	}

	cfgsub, err := config.NewSub()
	if err != nil {
		log.L().Fatal("Failed to new sub chain config.", zap.Error(err))
	}
	if cfgsub.Chain.ID != 0 {
		if err := svr.NewSubChainService(cfgsub); err != nil {
			log.L().Fatal("Failed to new sub chain.", zap.Error(err))
		}
	}

	itx.StartServer(svr, cfg)
}

func initLogger(cfg config.Config) {
	addr, err := cfg.BlockchainAddress()
	if err != nil {
		glog.Fatalln("Failed to get producer address from pub/kri key: ", err)
		return
	}
	if err := log.InitGlobal(cfg.Log, zap.Fields(
		zap.String("addr", addr.Bech32()),
		zap.String("networkAddress", fmt.Sprintf("%s:%d", cfg.Network.Host, cfg.Network.Port)),
		zap.String("nodeType", cfg.NodeType),
	)); err != nil {
		glog.Println("Cannot config global logger, use default one: ", err)
	}
}
