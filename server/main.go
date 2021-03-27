// Copyright (c) 2019 IoTeX Foundation
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
	glog "log"
	"os"
	"os/signal"
	"syscall"

	_ "go.uber.org/automaxprocs"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/probe"
	"github.com/iotexproject/iotex-core/server/itx"
)

func init() {
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr,
			"usage: server -config-path=[string]\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()
}

func main() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	signal.Notify(stop, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	livenessCtx, livenessCancel := context.WithCancel(context.Background())

	genesisCfg, err := genesis.New()
	if err != nil {
		glog.Fatalln("Failed to new genesis config.", zap.Error(err))
	}

	cfg, err := config.New()
	if err != nil {
		glog.Fatalln("Failed to new config.", zap.Error(err))
	}
	initLogger(cfg)

	if config.ExternChainID() == 0 {
		glog.Fatalln("Extern chain ID not set, call config.New() first")
	}

	cfg.Genesis = genesisCfg
	cfgToLog := cfg
	cfgToLog.Chain.ProducerPrivKey = ""
	log.S().Infof("Config in use: %+v", cfgToLog)

	// liveness start
	probeSvr := probe.New(cfg.System.HTTPStatsPort)
	if err := probeSvr.Start(ctx); err != nil {
		log.L().Fatal("Failed to start probe server.", zap.Error(err))
	}
	go func() {
		<-stop
		// start stopping
		cancel()
		<-stopped

		// liveness end
		if err := probeSvr.Stop(livenessCtx); err != nil {
			log.L().Error("Error when stopping probe server.", zap.Error(err))
		}
		livenessCancel()
	}()

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

	itx.StartServer(ctx, svr, probeSvr, cfg)
	close(stopped)
	<-livenessCtx.Done()
}

func initLogger(cfg config.Config) {
	addr := cfg.ProducerAddress()
	if err := log.InitLoggers(cfg.Log, cfg.SubLogs, zap.Fields(
		zap.String("ioAddr", addr.String()),
		zap.String("networkAddr", fmt.Sprintf("%s:%d", cfg.Network.Host, cfg.Network.Port)),
	)); err != nil {
		glog.Println("Cannot config global logger, use default one: ", err)
	}
}
