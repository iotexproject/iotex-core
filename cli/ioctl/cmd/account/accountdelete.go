// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"bytes"
	"fmt"
	"syscall"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// accountDeleteCmd represents the account delete command
var accountDeleteCmd = &cobra.Command{
	Use:   "delete (ALIAS|ADDRESS)",
	Short: "Delete an IoTeX account/address from wallet/config",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := accountDelete(args)
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

func accountDelete(args []string) (string, error) {
	addr, err := alias.Address(args[0])
	if err != nil {
		return "", err
	}
	account, err := address.FromString(addr)
	if err != nil {
		log.L().Error("failed to convert string into address", zap.Error(err))
		return "", err
	}
	ks := keystore.NewKeyStore(config.ReadConfig.Wallet,
		keystore.StandardScryptN, keystore.StandardScryptP)
	for _, v := range ks.Accounts() {
		if bytes.Equal(account.Bytes(), v.Address.Bytes()) {
			var confirm string
			fmt.Println("** This is an irreversible action!\n" +
				"Once an account is deleted, all the assets under this account may be lost!\n" +
				"Type 'YES' to continue, quit for anything else.")
			fmt.Scanf("%s", &confirm)
			if confirm != "YES" && confirm != "yes" {
				return "Quit", nil
			}

			fmt.Printf("Enter password #%s:\n", addr)
			bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
			if err != nil {
				log.L().Error("failed to get password", zap.Error(err))
				return "", err
			}
			password := string(bytePassword)
			if err := ks.Delete(v, password); err != nil {
				return "", err
			}
			return fmt.Sprintf("Account #%s has been deleted.", addr), nil
		}
	}
	return "", fmt.Errorf("account #%s not found", addr)
}
