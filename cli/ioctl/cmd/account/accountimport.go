// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"
	"io/ioutil"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/cli/ioctl/validator"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// accountImportCmd represents the account import command
var accountImportCmd = &cobra.Command{
	Use:   "import ALIAS",
	Short: "Import IoTeX private key into wallet",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(accountImport(args))
	},
}

func accountImport(args []string) string {
	// Validate inputs
	if err := validator.ValidateAlias(args[0]); err != nil {
		return err.Error()
	}
	alias := args[0]
	if addr, ok := config.ReadConfig.Aliases[alias]; ok {
		return fmt.Sprintf("alias \"%s\" has already used for %s.", alias, addr)
	}
	fmt.Printf("#%s: Enter your private key, which will not be exposed on the screen.\n", alias)
	privateKeyBytes, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.L().Error("failed to get private key", zap.Error(err))
		return err.Error()
	}
	privateKey := strings.TrimSpace(string(privateKeyBytes))
	for i := 0; i < len(privateKeyBytes); i++ {
		privateKeyBytes[i] = 0
	}
	addr, err := newAccountByKey(alias, privateKey, config.ReadConfig.Wallet)
	privateKey = ""
	if err != nil {
		return err.Error()
	}
	config.ReadConfig.Aliases[alias] = addr
	out, err := yaml.Marshal(&config.ReadConfig)
	if err != nil {
		return err.Error()
	}
	if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		return fmt.Sprintf("Failed to write to config file %s.", config.DefaultConfigFile)
	}
	return fmt.Sprintf(
		"New account #%s is created. Keep your password, or your will lose your private key.",
		alias,
	)
}
