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
	glog "log"
	"os"

	_ "go.uber.org/automaxprocs"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/tools/bot/config"
	"github.com/iotexproject/iotex-core/tools/bot/pkg/util/mailutil"
	"github.com/iotexproject/iotex-core/tools/bot/server/bot"
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
	cfg, err := config.New()
	if err != nil {
		glog.Fatalln("Failed to new config.", zap.Error(err))
	}
	initLogger(cfg)

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

	alert := &mailutil.Email{}
	b.Register(transfer)
	b.Register(xrc20)
	b.Register(multisend)
	b.RegisterAlert(alert)

	if err := b.Start(context.Background()); err != nil {
		log.L().Fatal("Failed to start server.", zap.Error(err))
	}
	defer b.Stop()
	select {}
}

func initLogger(cfg config.Config) {
	if err := log.InitLoggers(cfg.Log, cfg.SubLogs); err != nil {
		glog.Println("Cannot config global logger, use default one: ", err)
	}
}
