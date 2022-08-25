// Copyright (c) 2019 IoTeX Foundation
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
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	_listCmdShorts = map[config.Language]string{
		config.English: "List existing account for ioctl",
		config.Chinese: "列出ioctl中已存在的账户",
	}
)

// _accountListCmd represents the account list command
var _accountListCmd = &cobra.Command{
	Use:   "list",
	Short: config.TranslateInLang(_listCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := accountList()
		return output.PrintError(err)
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

	if CryptoSm2 {
		sm2Accounts, err := listSm2Account()
		if err != nil {
			return output.NewError(output.ReadFileError, "failed to get sm2 accounts", err)
		}
		for _, addr := range sm2Accounts {
			message.Accounts = append(message.Accounts, account{
				Address: addr,
				Alias:   aliases[addr],
			})
		}
	} else {
		ks := keystore.NewKeyStore(config.ReadConfig.Wallet,
			keystore.StandardScryptN, keystore.StandardScryptP)
		for _, v := range ks.Accounts() {
			addr, err := address.FromBytes(v.Address.Bytes())
			if err != nil {
				return output.NewError(output.ConvertError, "failed to convert bytes into address", err)
			}
			message.Accounts = append(message.Accounts, account{
				Address: addr.String(),
				Alias:   aliases[addr.String()],
			})
		}
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
