// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// This is a recovery tool that recovers a corrupted or missing state database.
// To use, run "make recover"
package main

import (
	"context"
	"flag"
	"fmt"
	glog "log"
	"os"

	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain/genesis"
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
			"usage: recover -config-path=[string]\n -recovery-height=[int]\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()
}

func main() {
	genesisCfg, err := genesis.New()
	if err != nil {
		glog.Fatalln("Failed to new genesis config.", zap.Error(err))
	}

	cfg, err := config.New()
	if err != nil {
		glog.Fatalln("Failed to new config.", zap.Error(err))
	}

	cfg.Genesis = genesisCfg

	log.S().Infof("Config in use: %+v", cfg)

	// create server
	svr, err := itx.NewServer(cfg)
	if err != nil {
		log.L().Fatal("Failed to create server.", zap.Error(err))
	}

	// recover chain and state
	bc := svr.ChainService(cfg.Chain.ID).Blockchain()
	if err := bc.Start(context.Background()); err == nil {
		log.L().Info("State DB status is normal.")
	}
	defer func() {
		if err := bc.Stop(context.Background()); err != nil {
			log.L().Fatal("Failed to stop blockchain")
		}
	}()

	if err := bc.RecoverChainAndState(uint64(recoveryHeight)); err != nil {
		log.L().Fatal("Failed to recover chain and state.", zap.Error(err))
	}
}
