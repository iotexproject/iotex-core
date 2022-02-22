// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"
	"strings"

	"github.com/iotexproject/iotex-address/address"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	listCmdShorts = map[config.Language]string{
		config.English: "List existing account for ioctl",
		config.Chinese: "列出ioctl中已存在的账户",
	}
	listCmdUses = map[config.Language]string{
		config.English: "list",
	}
)

// NewAccountList represents the account list command
func NewAccountList(c ioctl.Client) *cobra.Command {
	use, _ := c.SelectTranslation(listCmdUses)
	short, _ := c.SelectTranslation(listCmdShorts)
	al := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			listmessage := listMessage{}
			aliases := c.AliasMap()

			if c.IsCryptoSm2() {
				sm2Accounts, err := listSm2Account()
				if err != nil {
					return output.NewError(output.ReadFileError, "failed to get sm2 accounts", err)
				}
				for _, addr := range sm2Accounts {
					listmessage.Accounts = append(listmessage.Accounts, account{
						Address: addr,
						Alias:   aliases[addr],
					})
				}
			} else {
				ks := c.NewKeyStore()
				for _, v := range ks.Accounts() {
					addr, err := address.FromBytes(v.Address.Bytes())
					if err != nil {
						return output.NewError(output.ConvertError, "failed to convert bytes into address", err)
					}
					listmessage.Accounts = append(listmessage.Accounts, account{
						Address: addr.String(),
						Alias:   aliases[addr.String()],
					})
				}
			}

			fmt.Println(listmessage.String())

			return nil
		},
	}
	return al
}

type listMessage struct {
	Accounts []account `json:"accounts"`
}

type account struct {
	Address string `json:"address"`
	Alias   string `json:"alias"`
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
