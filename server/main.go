// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// Usage:
//   make build
//   ./bin/server -config=./config.yaml
//

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/explorer"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/rpcservice"
	"github.com/iotexproject/iotex-core/server/itx"
)

var configFile = flag.String("config", "./config.yaml", "specify configuration file path")

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr,
			"usage: server -config=[string]\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()
}

func main() {
	cfg, err := config.LoadConfigWithPath(*configFile)

	if err != nil {
		os.Exit(1)
	}

	// create and start the node
	svr := itx.NewServer(*cfg)
	if err := svr.Init(); err != nil {
		os.Exit(1)
	}
	if err := svr.Start(); err != nil {
		os.Exit(1)
	}
	defer svr.Stop()

	// start the chain server for Tx injection
	if cfg.RPC != (config.RPC{}) {
		bcb := func(msg proto.Message) error {
			return svr.P2p().Broadcast(msg)
		}
		cs := rpcservice.NewChainServer(cfg.RPC, svr.Bc(), svr.Dp(), bcb)
		if cs == nil {
			os.Exit(1)
		}
		cs.Start()
		defer cs.Stop()
	}

	select {}

	if cfg.Explorer.StartExplorer {
		isTest := cfg.Explorer.IsTest
		httpPort := cfg.Explorer.Addr

		flag.Parse()
		env := os.Getenv("APP_ENV")
		if env == "development" {
			isTest = true
		}
		if isTest {
			logger.Warn().Msg("Using test server with fake data...")
		}
		explorer.StartJSONServer(svr.Bc(), isTest, httpPort)
	}
}
