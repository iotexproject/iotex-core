// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"go/build"
	"os"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/explorer"
	eidl "github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/logger"
)

const (
	yamlPath  = "/src/github.com/iotexproject/iotex-core/e2etests/config_local_delegate.yaml"
	localhost = "http://127.0.0.1:"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "iotc [command] [flags]",
	Short: "Command-line interface for IoTeX blockchain",
	Long: `iotc is a command-line interface which queries the IoTeX blockchain and can return a variety 
of useful information about the state of the blockchain or given account.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// getClientAndCfg gets the explorer client and config file
func getClientAndCfg() (eidl.Explorer, *config.Config) {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		logger.Error().Msg("please set GOPATH environment variable")
		gopath = build.Default.GOPATH
	}
	configFile := gopath + yamlPath
	cfg, err := config.LoadConfigWithPath(configFile)
	if err != nil {
		logger.Error().Err(err).Msg("cannot access config file")
	}
	port := cfg.Explorer.Addr
	client := explorer.NewExplorerProxy(localhost + port)

	return client, cfg
}
