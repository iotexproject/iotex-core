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
)

// aliasRemoveCmd represents the alias remove command
var aliasRemoveCmd = &cobra.Command{
	Use:   "remove ALIAS",
	Short: "Remove alias",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(remove(args[0]))
	},
}

// remove removes alias
func remove(arg string) string {
	if err := validator.ValidateAlias(arg); err != nil {
		return err.Error()
	}
	alias := arg
	delete(config.ReadConfig.Aliases, alias)
	out, err := yaml.Marshal(&config.ReadConfig)
	if err != nil {
		return err.Error()
	}
	if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		return fmt.Sprintf("Failed to write to config file %s.", config.DefaultConfigFile)
	}
	return alias + " is removed"
}
