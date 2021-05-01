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

	"github.com/iotexproject/go-pkgs/hash"
	_ "go.uber.org/automaxprocs"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain/block"
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
	// set genesis timestamp
	genesis.SetGenesisTimestamp(genesisCfg.Timestamp)
	if genesis.Timestamp() == 0 {
		glog.Fatalln("Genesis timestamp is not set, call genesis.New() first")
	}

	cfg, err := config.New()
	if err != nil {
		glog.Fatalln("Failed to new config.", zap.Error(err))
	}
	if err = initLogger(cfg); err != nil {
		glog.Fatalln("Cannot config global logger, use default one: ", zap.Error(err))
	}

	// populdate chain ID
	config.SetEVMNetworkID(cfg.Chain.EVMNetworkID)
	if config.EVMNetworkID() == 0 {
		glog.Fatalln("EVM Network ID is not set, call config.New() first")
	}

	// load genesis block's hash
	block.LoadGenesisHash()
	if block.GenesisHash() == hash.ZeroHash256 {
		glog.Fatalln("Genesis hash is not set, call block.LoadGenesisHash() first")
	}

	cfg.Genesis = genesisCfg
	cfgToLog := cfg
	cfgToLog.Chain.ProducerPrivKey = ""
	cfgToLog.Network.MasterKey = ""
	log.S().Infof("Config in use: %+v", cfgToLog)
	log.S().Infof("EVM Network ID: %d", config.EVMNetworkID())
	log.S().Infof("Genesis hash: %x", block.GenesisHash())

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

func initLogger(cfg config.Config) error {
	addr := cfg.ProducerAddress()
	return log.InitLoggers(cfg.Log, cfg.SubLogs, zap.Fields(
		zap.String("ioAddr", addr.String()),
	))
}
