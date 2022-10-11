// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// This is a recovery tool that recovers a corrupted or missing state database.
// To use, run "make recover"
package main

import (
	"flag"
	"fmt"
	"os"

	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state/factory"
)

var (
	// overwritePath is the path to the config file which overwrite default values
	_overwritePath string
	// secretPath is the path to the  config file store secret values
	_secretPath string
)

func init() {
	flag.StringVar(&_overwritePath, "config-path", "", "Config path")
	flag.StringVar(&_secretPath, "secret-path", "", "Secret path")
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "usage: readtip -config-path=[string]\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()
}

func main() {
	cfg, err := config.New([]string{_overwritePath, _secretPath}, []string{})
	if err != nil {
		log.S().Panic("failed to new config.", zap.Error(err))
	}
	cfg.DB.ReadOnly = true
	store, err := db.CreateKVStore(cfg.DB, cfg.Chain.TrieDBPath)
	if err != nil {
		log.S().Panic("failed to load state db", zap.Error(err))
	}
	h, err := store.Get(factory.AccountKVNamespace, []byte(factory.CurrentHeightKey))
	if err != nil {
		log.S().Panic("failed to read state db", zap.Error(err))
	}
	fmt.Println(byteutil.BytesToUint64(h))
}
