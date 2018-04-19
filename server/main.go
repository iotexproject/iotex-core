// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// Usage:
//   make build
//   ./bin/server -stderrthreshold=WARNING -log_dir=./log -config=./config.yaml
//

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/server/run"
)

var configFile = flag.String("config", "./config.yaml", "specify configuration file path")

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr,
			"usage: server -stderrthreshold=[INFO|WARN|FATAL] -log_dir=[string] -config=[string]\n")
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

	run.Run(cfg, nil)
}
