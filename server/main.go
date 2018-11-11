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
	"os"

	_ "net/http/pprof"

	_ "go.uber.org/automaxprocs"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/logger"
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
		logger.Fatal().Err(err).Msg("Failed to new config.")
	}

	initLogger(cfg)

	// create and start the node
	svr, err := itx.NewServer(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create server.")
	}

	cfgsub, err := config.NewSub()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to new sub chain config.")
	}
	// Ifminicluster.go the sub-chain config is not empty
	if cfgsub.Chain.ID != 0 {
		if err := svr.NewChainService(cfgsub); err != nil {
			logger.Fatal().Err(err).Msg("Failed to new sub chain.")
		}
	}

	itx.StartServer(svr, cfg)
}

func initLogger(cfg config.Config) {
	addr, err := cfg.BlockchainAddress()
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
				Str("iotxAddr", addr.IotxAddress()).
				Str("networkAddress", fmt.Sprintf("%s:%d", cfg.Network.Host, cfg.Network.Port)).
				Str("nodeType", cfg.NodeType).Logger(),
		)
	}
}
