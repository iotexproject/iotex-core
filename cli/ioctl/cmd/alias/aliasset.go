// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package alias

import (
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/cli/ioctl/validator"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// aliasSetCmd represents the alias set command
var aliasSetCmd = &cobra.Command{
	Use:   "set ALIAS ADDRESS",
	Short: "Set alias for address",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		set(args)
	},
}

// set sets alias
func set(args []string) {
	if err := validator.ValidateAlias(args[0]); err != nil {
		log.L().Panic(err.Error())
	}
	alias := args[0]
	if err := validator.ValidateAddress(args[1]); err != nil {
		log.L().Panic(err.Error())
	}
	addr := args[1]
	config.ReadConfig.Aliases[alias] = addr
	out, err := yaml.Marshal(&config.ReadConfig)
	if err != nil {
		log.L().Panic(err.Error())
	}
	if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		log.L().Panic(fmt.Sprintf("Failed to write to config file %s.", config.DefaultConfigFile))
	}
}
