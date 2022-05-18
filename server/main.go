// Copyright (c) 2022 IoTeX Foundation
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
	"strings"

	_ "go.uber.org/automaxprocs"
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

}
