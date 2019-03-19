// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/cli/ioctl/validator"
)

// accountNameCmd represents the account name command
var accountNameCmd = &cobra.Command{
	Use:   "name ADDRESS NAME",
	Short: "Name an IoTeX address in config",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(name(args))
	},
}

func name(args []string) string {
	// Validate inputs
	if err := validator.ValidateAddress(args[0]); err != nil {
		return err.Error()
	}
	address := args[0]
	if err := validator.ValidateName(args[1]); err != nil {
		return err.Error()
	}
	name := args[1]
	cfg, err := config.LoadConfig()
	if err != nil {
		return err.Error()
	}
	for n, addr := range cfg.AccountList {
		if n == name || addr == address {
			return fmt.Sprintf("An account #%s:%s already exists.", n, addr)
		}
	}
	for n, addr := range cfg.NameList {
		if n == name || addr == address {
			return fmt.Sprintf("Name \"%s\" has been used for %s.", n, addr)
		}
	}

	cfg.NameList[name] = address
	out, err := yaml.Marshal(&cfg)
	if err != nil {
		return err.Error()
	}
	if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		return fmt.Sprintf("Failed to write to config file %s.", config.DefaultConfigFile)
	}
	return fmt.Sprintf("Succeed to name %s \"%s\".", address, name)
}
