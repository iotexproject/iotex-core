// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

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
	"strings"
	"syscall"

	"github.com/iotexproject/go-pkgs/hash"
	"go.uber.org/automaxprocs/maxprocs"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/db/trie/mptrie"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/probe"
	"github.com/iotexproject/iotex-core/v2/pkg/recovery"
	"github.com/iotexproject/iotex-core/v2/server/itx"
)

/**
 * overwritePath is the path to the config file which overwrite default values
 * secretPath is the path to the  config file store secret values
 */
var (
	_genesisPath   string
	_overwritePath string
	_secretPath    string
	_subChainPath  string
	_plugins       strs
)

type strs []string

func (ss *strs) String() string {
	return strings.Join(*ss, ",")
}

func (ss *strs) Set(str string) error {
	*ss = append(*ss, str)
	return nil
}

func init() {
	// set max number of CPUs, disable log printing
	maxprocs.Set(maxprocs.Logger(nil))
	flag.StringVar(&_genesisPath, "genesis-path", "", "Genesis path")
	flag.StringVar(&_overwritePath, "config-path", "", "Config path")
	flag.StringVar(&_secretPath, "secret-path", "", "Secret path")
	flag.StringVar(&_subChainPath, "sub-config-path", "", "Sub chain Config path")
	flag.Var(&_plugins, "plugin", "Plugin of the node")
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

	genesisCfg, err := genesis.New(_genesisPath)
	if err != nil {
		glog.Fatalln("Failed to new genesis config.", zap.Error(err))
	}
	// set genesis timestamp
	genesis.SetGenesisTimestamp(genesisCfg.Timestamp)
	if genesis.Timestamp() == 0 {
		glog.Fatalln("Genesis timestamp is not set, call genesis.New() first")
	}
	// load genesis block's hash
	block.LoadGenesisHash(&genesisCfg)
	if block.GenesisHash() == hash.ZeroHash256 {
		glog.Fatalln("Genesis hash is not set, call block.LoadGenesisHash() first")
	}

	cfg, err := config.New([]string{_overwritePath, _secretPath}, _plugins)
	if err != nil {
		glog.Fatalln("Failed to new config.", zap.Error(err))
	}
	if err = initLogger(cfg); err != nil {
		glog.Fatalln("Cannot config global logger, use default one: ", zap.Error(err))
	}

	if err = recovery.SetCrashlogDir(cfg.System.SystemLogDBPath); err != nil {
		glog.Fatalln("Failed to set directory of crashlog: ", zap.Error(err))
	}
	defer recovery.Recover()

	// check EVM network ID and chain ID
	if cfg.Chain.EVMNetworkID == 0 || cfg.Chain.ID == 0 {
		glog.Fatalln("EVM Network ID or Chain ID is not set, call config.New() first")
	}

	cfg.Genesis = genesisCfg
	cfgToLog := cfg
	cfgToLog.Chain.ProducerPrivKey = ""
	cfgToLog.Network.MasterKey = ""
	log.S().Infof("Config in use: %+v", cfgToLog)
	log.S().Infof("EVM Network ID: %d, Chain ID: %d", cfg.Chain.EVMNetworkID, cfg.Chain.ID)
	log.S().Infof("Genesis timestamp: %d", genesisCfg.Timestamp)
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

	if cfg.System.MptrieLogPath != "" {
		if err = mptrie.OpenLogDB(cfg.System.MptrieLogPath); err != nil {
			log.L().Fatal("Failed to open mptrie log DB.", zap.Error(err))
		}
		defer func() {
			if err = mptrie.CloseLogDB(); err != nil {
				log.L().Error("Failed to close mptrie log DB.", zap.Error(err))
			}
		}()
	}
	// create and start the node
	svr, err := itx.NewServer(cfg)
	if err != nil {
		log.L().Fatal("Failed to create server.", zap.Error(err))
	}

	var cfgsub config.Config
	if _subChainPath != "" {
		cfgsub, err = config.NewSub([]string{_secretPath, _subChainPath})
		if err != nil {
			log.L().Fatal("Failed to new sub chain config.", zap.Error(err))
		}
	} else {
		cfgsub = config.Config{}
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
	addrs := cfg.Chain.ProducerAddress()
	ss := []string{}
	for _, addr := range addrs {
		ss = append(ss, addr.String())
	}
	return log.InitLoggers(cfg.Log, cfg.SubLogs, zap.AddCaller(), zap.Fields(
		zap.String("ioAddr", strings.Join(ss, ",")),
	))
}
