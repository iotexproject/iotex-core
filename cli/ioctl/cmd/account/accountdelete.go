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
	"os"
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
	Use:   "delete name/address",
	Short: "delete an IoTeX keypair from wallet",
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
	name, addr := "", ""
	for nameInList, addrInList := range cfg.AccountList {
		if nameInList == args[0] || addrInList == args[0] {
			name, addr = nameInList, addrInList
			delete(cfg.AccountList, name)
		}
	}
	if len(addr) == 0 {
		return "can't find account from #" + args[0]
	}
	account, err := address.FromString(addr)
	if err != nil {
		return err.Error()
	}
	wallet := config.Get("wallet")

	fmt.Printf("Enter password #%s:\n", name)
	bytePassword, err := terminal.ReadPassword(syscall.Stdin)
	if err != nil {
		log.L().Error("fail to get password", zap.Error(err))
		return err.Error()
	}
	password := string(bytePassword)
	ks := keystore.NewKeyStore(wallet, keystore.StandardScryptN, keystore.StandardScryptP)
	for _, v := range ks.Accounts() {
		if bytes.Equal(account.Bytes(), v.Address.Bytes()) {
			if err := ks.Delete(v, password); err != nil {
				return err.Error()
			}
			fmt.Printf("delete: %s \n", v.URL)
		}
	}
	out, err := yaml.Marshal(&cfg)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		fmt.Printf("Failed to write to config file %s.", config.DefaultConfigFile)
		os.Exit(1)
	}
	return fmt.Sprintf("Account #%s:%s has been deleted", name, addr)
}
