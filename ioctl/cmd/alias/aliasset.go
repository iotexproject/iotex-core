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
	yaml "gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/validator"
)

// aliasSetCmd represents the alias set command
var aliasSetCmd = &cobra.Command{
	Use:   "set ALIAS ADDRESS",
	Short: "Set alias for address",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := set(args)
		return err
	},
}

// set sets alias
func set(args []string) error {
	if err := validator.ValidateAlias(args[0]); err != nil {
		return output.PrintError(output.ValidationError, err.Error())
	}
	alias := args[0]
	if err := validator.ValidateAddress(args[1]); err != nil {
		return output.PrintError(output.ValidationError, err.Error())
	}
	addr := args[1]
	aliases := GetAliasMap()
	for aliases[addr] != "" {
		delete(config.ReadConfig.Aliases, aliases[addr])
		aliases = GetAliasMap()
	}
	config.ReadConfig.Aliases[alias] = addr
	out, err := yaml.Marshal(&config.ReadConfig)
	if err != nil {
		return output.PrintError(output.SerializationError, err.Error())
	}
	if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		return output.PrintError(output.WriteFileError,
			fmt.Sprintf("failed to write to config file %s", config.DefaultConfigFile))
	}
	message := output.StringMessage("set")
	fmt.Println(message.String())
	return nil
}
