// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"syscall"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// accountDeleteCmd represents the account delete command
var accountDeleteCmd = &cobra.Command{
	Use:   "delete (NAME|ADDRESS)",
	Short: "Delete an IoTeX account/address from wallet/config",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(accountDelete(args))
	},
}

func accountDelete(args []string) string {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err.Error()
	}

	for name, addr := range cfg.NameList {
		if name == args[0] || addr == args[0] {
			delete(cfg.NameList, name)
			out, err := yaml.Marshal(&cfg)
			if err != nil {
				return err.Error()
			}
			if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
				return "Failed to write to config file %s." + config.DefaultConfigFile
			}
			return fmt.Sprintf("Name \"%s\":%s has been removed", name, addr)
		}
	}

	var confirm string
	fmt.Println("** This is an irreversible action!\n" +
		"Once an account is deleted, all the assets under this account may be lost!\n" +
		"Type 'YES' to continue, quit for anything else.")
	fmt.Scanf("%s", &confirm)
	if confirm != "YES" && confirm != "yes" {
		return "Quit"
	}

	found := false
	name, addr := "", ""
	var account address.Address
	for name, addr = range cfg.AccountList {
		if name == args[0] || addr == args[0] {
			account, err = address.FromString(addr)
			if err != nil {
				log.L().Error(fmt.Sprintf("Account #%s:%s is not valid", name, addr),
					zap.Error(err))
				return err.Error()
			}
			delete(cfg.AccountList, name)
			found = true
			break
		}
	}
	if !found {
		account, err = address.FromString(args[0])
		if err != nil {
			return fmt.Sprintf("Account #%s not found", args[0])
		}
	}
	ksFound := false
	wallet := cfg.Wallet
	ks := keystore.NewKeyStore(wallet, keystore.StandardScryptN, keystore.StandardScryptP)
	for _, v := range ks.Accounts() {
		if bytes.Equal(account.Bytes(), v.Address.Bytes()) {
			fmt.Printf("Enter password #%s:\n", name)
			bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
			if err != nil {
				log.L().Error("fail to get password", zap.Error(err))
				return err.Error()
			}
			password := string(bytePassword)
			if err := ks.Delete(v, password); err != nil {
				return err.Error()
			}
			ksFound = true
			break
		}
	}
	if found {
		out, err := yaml.Marshal(&cfg)
		if err != nil {
			return err.Error()
		}
		if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
			return "Failed to write to config file %s." + config.DefaultConfigFile
		}
		return fmt.Sprintf("Account #%s:%s has been deleted.", name, addr)
	}
	if !ksFound {
		return fmt.Sprintf("Account #%s not found", args[0])
	}
	return fmt.Sprintf("Account #%s has been deleted.", args[0])
}
