// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// Usage:
//   make build
//   ./bin/server -config=./config.yaml
//

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/explorer"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/routine"
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

	ctx := context.Background()
	// create and start the node
	svr := itx.NewServer(*cfg)
	if err := svr.Start(ctx); err != nil {
		os.Exit(1)
	}
	defer svr.Stop(ctx)

	if cfg.System.HeartbeatInterval > 0 {
		task := routine.NewRecurringTask(itx.NewHeartbeatHandler(svr), cfg.System.HeartbeatInterval)
		if err := task.Start(ctx); err != nil {
			logger.Panic().Err(err)
		}
		defer func() {
			if err := task.Stop(ctx); err != nil {
				logger.Panic().Err(err)
			}
		}()
	}

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

	if cfg.Explorer.Enabled {
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
		explorer.StartJSONServer(svr.Bc(), svr.Cs(), isTest, httpPort, cfg.Explorer.TpsWindow)
	}

	select {}
}
