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
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/routine"
	"github.com/iotexproject/iotex-core/server/itx"
)

func init() {
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr,
			"usage: server -config=[string]\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()
}

func main() {
	cfg, err := config.New()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to new config.")
		return
	}

	initLogger(cfg)

	ctx := context.Background()
	// create and start the node
	svr := itx.NewServer(cfg)
	if err := svr.Start(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Fail to start server.")
		return
	}
	defer func() {
		err := svr.Stop(ctx)
		if err != nil {
			logger.Error().Err(err)
		}
	}()

	if cfg.System.HeartbeatInterval > 0 {
		task := routine.NewRecurringTask(itx.NewHeartbeatHandler(svr).Log, cfg.System.HeartbeatInterval)
		if err := task.Start(ctx); err != nil {
			logger.Panic().Err(err)
		}
		defer func() {
			if err := task.Stop(ctx); err != nil {
				logger.Panic().Err(err)
			}
		}()
	}

	if cfg.System.HTTPProfilingPort > 0 {
		go func() {
			if err := http.ListenAndServe(
				fmt.Sprintf(":%d", cfg.System.HTTPProfilingPort),
				nil,
			); err != nil {
				logger.Error().Err(err).Msg("error when serving performance profiling data")
			}
		}()
	}
	select {}
}

func initLogger(cfg *config.Config) {
	iotxAddr, err := cfg.ProducerAddr()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to get producer address from pub/kri key.")
		return
	}
	l, err := logger.New()
	if err != nil {
		logger.Warn().Err(err).Msg("Cannot config logger, use default one.")
	} else {
		logger.SetLogger(
			l.With().
				Str("iotexAddr", iotxAddr.RawAddress).
				Str("networkAddress", fmt.Sprintf("%s:%d", cfg.Network.Host, cfg.Network.Port)).
				Str("nodeType", cfg.NodeType).Logger(),
		)
	}
}
