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

var (
	// accountImportCmd represents the account import command
	accountImportCmd = &cobra.Command{
		Use:   "import",
		Short: "Import IoTeX private key or keystore into wallet",
	}
	// accountImportKeyCmd represents the account import key command
	accountImportKeyCmd = &cobra.Command{
		Use:   "key ALIAS",
		Short: "Import IoTeX private key into wallet",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			output, err := accountImportKey(args)
			if err == nil {
				fmt.Println(output)
			}
			return err
		},
	}
	// accountImportKeyCmd represents the account import keystore command
	accountImportKeyStoreCmd = &cobra.Command{
		Use:   "keystore ALIAS FILEPATH",
		Short: "Import IoTeX keystore into wallet",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			output, err := accountImportKeyStore(args)
			if err == nil {
				fmt.Println(output)
			}
			return err
		},
	}
)

func init() {
	accountImportCmd.AddCommand(accountImportKeyCmd)
	accountImportCmd.AddCommand(accountImportKeyStoreCmd)
}
func validataAlias(alias string) error {
	if err := validator.ValidateAlias(alias); err != nil {
		return err
	}
	if addr, ok := config.ReadConfig.Aliases[alias]; ok {
		return fmt.Errorf("alias \"%s\" has already used for %s", alias, addr)
	}
	return nil
}
func writeToFile(alias, addr string) (string, error) {
	config.ReadConfig.Aliases[alias] = addr
	out, err := yaml.Marshal(&config.ReadConfig)
	if err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		return "", fmt.Errorf("failed to write to config file %s", config.DefaultConfigFile)
	}
	return fmt.Sprintf(
		"New account #%s is created. Keep your password, or your will lose your private key.",
		alias), nil
}
func readPasswordFromStdin() (string, error) {
	passwordBytes, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.L().Error("failed to get password", zap.Error(err))
		return "", err
	}
	password := strings.TrimSpace(string(passwordBytes))
	for i := 0; i < len(passwordBytes); i++ {
		passwordBytes[i] = 0
	}
	return password, nil
}
func accountImportKey(args []string) (string, error) {
	// Validate inputs
	alias := args[0]
	err := validataAlias(alias)
	if err != nil {
		return "", err
	}
	fmt.Printf("#%s: Enter your private key, which will not be exposed on the screen.\n", alias)
	privateKey, err := readPasswordFromStdin()
	if err != nil {
		return "", nil
	}
	addr, err := newAccountByKey(alias, privateKey, config.ReadConfig.Wallet)
	if err != nil {
		return "", err
	}
	return writeToFile(alias, addr)
}
func accountImportKeyStore(args []string) (string, error) {
	// Validate inputs
	alias := args[0]
	err := validataAlias(alias)
	if err != nil {
		return "", err
	}
	fmt.Printf("#%s: Enter your password of keystore, which will not be exposed on the screen.\n", alias)
	password, err := readPasswordFromStdin()
	if err != nil {
		return "", nil
	}
	addr, err := newAccountByKeyStore(alias, password, args[1], config.ReadConfig.Wallet)
	if err != nil {
		return "", err
	}
	return writeToFile(alias, addr)
}
