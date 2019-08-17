// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	_ "go.uber.org/automaxprocs"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/tools/bot/config"
	"github.com/iotexproject/iotex-core/tools/bot/server/bot"
)

func init() {
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr,
			"usage: bot -config-path=[string]\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()
}

func main() {
	cfg, err := config.New()
	if err != nil {
		fmt.Println("Failed to new config.", zap.Error(err))
		return
	}
	err = initLogger(cfg)
	if err != nil {
		return
	}
	b, err := bot.NewServer(cfg)
	if err != nil {
		log.L().Fatal("new server:", zap.Error(err))
	}

	// transfer
	transfer, err := bot.NewTransfer(cfg, "transfer")
	if err != nil {
		log.L().Fatal("new transfer:", zap.Error(err))
	}

	// xrc20
	xrc20, err := bot.NewXrc20(cfg, "xrc20")
	if err != nil {
		log.L().Fatal("new xrc20 transfer:", zap.Error(err))
	}

	// multisend
	multisend, err := bot.NewExecution(cfg, "multisend")
	if err != nil {
		log.L().Fatal("new multisend:", zap.Error(err))
	}

	err = b.Register(transfer)
	if err != nil {
		log.L().Fatal("Register transfer:", zap.Error(err))
	}
	err = b.Register(xrc20)
	if err != nil {
		log.L().Fatal("Register xrc20 transfer:", zap.Error(err))
	}
	err = b.Register(multisend)
	if err != nil {
		log.L().Fatal("Register multisend:", zap.Error(err))
	}

	if err := b.Start(context.Background()); err != nil {
		log.L().Fatal("Failed to start server.", zap.Error(err))
	}
	defer b.Stop()
	select {}
}

func initLogger(cfg config.Config) error {
	if err := log.InitLoggers(cfg.Log, cfg.SubLogs); err != nil {
		fmt.Println("Cannot config global logger, use default one: ", err)
		return err
	}
	return nil
}
