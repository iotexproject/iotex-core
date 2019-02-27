// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package wallet

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
)

var password string

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
	walletCreateCmd.Flags().StringVarP(&password, "password", "p", "", "password for wallet")
	walletCreateCmd.MarkFlagRequired("password")
}

func walletCreate() string {
	ks := keystore.NewKeyStore(config.ConfigDir, keystore.StandardScryptN, keystore.StandardScryptP)
	account, err := ks.NewAccount(password)
	if err != nil {
		return err.Error()
	}
	addr, _ := address.FromBytes(account.Address.Bytes())
	return "New wallet \"" + addr.String() + "\" created, password = " + password +
		"\n**Remember to save your password. The wallet will be lost if you forgot the password!!"
}
