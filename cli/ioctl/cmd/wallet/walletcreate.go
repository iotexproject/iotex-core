// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package wallet

import (
	"fmt"
	"io/ioutil"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
)

var (
	name, password string
)

// walletCreateCmd represents the wallet create command
var walletCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create new wallet for ioctl",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(walletCreate())
	},
}

func init() {
	walletCreateCmd.Flags().StringVarP(&name, "name", "n", "", "name for wallet")
	walletCreateCmd.Flags().StringVarP(&password, "password", "p", "", "password for wallet")
	walletCreateCmd.MarkFlagRequired("name")
	walletCreateCmd.MarkFlagRequired("password")
}

func walletCreate() string {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err.Error()
	}
	if _, ok := cfg.WalletList[name]; ok {
		return fmt.Sprintf("A wallet named \"%s\" already exists.", name)
	}
	addr, err := newWallet()
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
		"New wallet \"%s\" is created. Keep your password, or the wallet will not be able to unlock.",
		name,
	)
}

func newWallet() (string, error) {
	ks := keystore.NewKeyStore(config.ConfigDir, keystore.StandardScryptN, keystore.StandardScryptP)
	account, err := ks.NewAccount(password)
	if err != nil {
		return "", err
	}
	addr, _ := address.FromBytes(account.Address.Bytes())
	return addr.String(), nil
}
