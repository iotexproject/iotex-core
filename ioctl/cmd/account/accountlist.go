// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// accountListCmd represents the account list command
var accountListCmd = &cobra.Command{
	Use:   "list",
	Short: "List existing account for ioctl",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := accountList()
		return err
	},
}

type listMessage struct {
	Accounts []account `json:"accounts"`
}

type account struct {
	Address string `json:"address"`
	Alias   string `json:"alias"`
}

func accountList() error {
	message := listMessage{}
	aliases := alias.GetAliasMap()
	ks := keystore.NewKeyStore(config.ReadConfig.Wallet,
		keystore.StandardScryptN, keystore.StandardScryptP)
	for _, v := range ks.Accounts() {
		address, err := address.FromBytes(v.Address.Bytes())
		if err != nil {
			return output.PrintError(output.ConvertError, "failed to convert bytes into address")
		}
		message.Accounts = append(message.Accounts, account{
			Address: address.String(),
			Alias:   aliases[address.String()],
		})
	}
	fmt.Println(message.String())
	return nil
}

func (m *listMessage) String() string {
	if output.Format == "" {
		lines := make([]string, 0)
		for _, account := range m.Accounts {
			line := account.Address
			if account.Alias != "" {
				line += " - " + account.Alias
			}
			lines = append(lines, line)
		}
		return strings.Join(lines, "\n")
	}
	return output.FormatString(output.Result, m)
}
