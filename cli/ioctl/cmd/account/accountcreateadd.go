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

// accountCreateAddCmd represents the account create command
var accountCreateAddCmd = &cobra.Command{
	Use:   "createadd name",
	Short: "Create new account for ioctl",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(accountCreateAdd(args))
	},
}

func accountCreateAdd(args []string) string {
	// Validate inputs
	if err := validator.ValidateName(args[0]); err != nil {
		return err.Error()
	}
	name := args[0]
	cfg, err := config.LoadConfig()
	if err != nil {
		return err.Error()
	}
	if _, ok := cfg.AccountList[name]; ok {
		return fmt.Sprintf("A account named \"%s\" already exists.", name)
	}
	wallet := cfg.Wallet
	addr, err := newAccount(name, wallet)
	if err != nil {
		return err.Error()
	}
	cfg.AccountList[name] = addr
	out, err := yaml.Marshal(&cfg)
	if err != nil {
		return err.Error()
	}
	if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		return fmt.Sprintf("Failed to write to config file %s.", config.DefaultConfigFile)
	}
	return fmt.Sprintf(
		"New account \"%s\" is created. Keep your password, or your will lose your private key.",
		name,
	)
}
