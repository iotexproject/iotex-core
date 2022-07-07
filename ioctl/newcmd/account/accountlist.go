// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"strings"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	_listCmdShorts = map[config.Language]string{
		config.English: "List existing account for ioctl",
		config.Chinese: "列出ioctl中已存在的账户",
	}
	_listCmdUses = map[config.Language]string{
		config.English: "list",
		config.Chinese: "list",
	}
)

// NewAccountList represents the account list command
func NewAccountList(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_listCmdUses)
	short, _ := client.SelectTranslation(_listCmdShorts)

	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			listmessage := listMessage{}
			aliases := client.AliasMap()

			if client.IsCryptoSm2() {
				sm2Accounts, err := listSm2Account(client)
				if err != nil {
					return errors.Wrap(err, "failed to get sm2 accounts")
				}
				for _, addr := range sm2Accounts {
					listmessage.Accounts = append(listmessage.Accounts, account{
						Address: addr,
						Alias:   aliases[addr],
					})
				}
			} else {
				ks := client.NewKeyStore()
				for _, v := range ks.Accounts() {
					addr, err := address.FromBytes(v.Address.Bytes())
					if err != nil {
						return errors.Wrap(err, "failed to convert bytes into address")
					}
					listmessage.Accounts = append(listmessage.Accounts, account{
						Address: addr.String(),
						Alias:   aliases[addr.String()],
					})
				}
			}
			cmd.Println(listmessage.String())
			return nil
		},
	}
}

type listMessage struct {
	Accounts []account `json:"accounts"`
}

type account struct {
	Address string `json:"address"`
	Alias   string `json:"alias"`
}

func (m *listMessage) String() string {
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
