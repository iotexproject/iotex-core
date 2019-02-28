// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/howeyc/gopass"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/pkg/log"
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
	name := args[0]
	cfg, err := config.LoadConfig()
	if err != nil {
		return err.Error()
	}
	if _, ok := cfg.WalletList[name]; ok {
		return fmt.Sprintf("A wallet named \"%s\" already exists.", name)
	}
	addr, err := newAccount(name)
	if err != nil {
		return err.Error()
	}
	cfg.WalletList[name] = addr
	out, err := yaml.Marshal(&cfg)
	if err != nil {
		return err.Error()
	}
	if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		return fmt.Sprintf("Failed to write to config file %s.", config.DefaultConfigFile)
	}
	return fmt.Sprintf(
		"New wallet \"%s\" is created. Keep your password, or your will lose your private key.",
		name,
	)
}

func newAccount(name string) (string, error) {
	fmt.Printf("#%s: Set password\n", name)
	passwordBytes, err := gopass.GetPasswd()
	if err != nil {
		log.L().Error("fail to get password", zap.Error(err))
		return "", err
	}
	password := string(passwordBytes)
	fmt.Printf("#%s: Enter password again\n", name)
	passwordCheck, err := gopass.GetPasswd()
	if err != nil {
		log.L().Error("fail to get password", zap.Error(err))
		return "", err
	}
	if password != string(passwordCheck) {
		return "", errors.New("password doesn't match")
	}
	ks := keystore.NewKeyStore(config.ConfigDir, keystore.StandardScryptN, keystore.StandardScryptP)
	account, err := ks.NewAccount(password)
	if err != nil {
		return "", err
	}
	addr, _ := address.FromBytes(account.Address.Bytes())
	return addr.String(), nil
}
