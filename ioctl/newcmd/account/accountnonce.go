// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	nonceCmdShorts = map[config.Language]string{
		config.English: "Get nonce of an account",
		config.Chinese: "获取账户的nonce值",
	}
	nonceCmdUses = map[config.Language]string{
		config.English: "nonce [ALIAS|ADDRESS]",
		config.Chinese: "nonce [别名|地址]",
	}
)

// NewAccountNonce represents the account nonce command
func NewAccountNonce(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(nonceCmdUses)
	short, _ := client.SelectTranslation(nonceCmdShorts)
	failToGetAddress, _ := client.SelectTranslation(failToGetAddress)
	failToGetAccountMeta, _ := client.SelectTranslation(failToGetAccountMeta)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			arg := ""
			if len(args) == 1 {
				arg = args[0]
			}

			addr, err := client.GetAddress(arg)
			if err != nil {
				return errors.Wrap(err, failToGetAddress)
			}
			accountMeta, err := AccountMeta(client, addr)
			if err != nil {
				return errors.Wrap(err, failToGetAccountMeta)
			}

			message := nonceMessage{
				Address:      addr,
				Nonce:        int(accountMeta.Nonce),
				PendingNonce: int(accountMeta.PendingNonce),
			}
			client.PrintInfo(message.String())
			return nil
		},
	}
	return cmd
}

type nonceMessage struct {
	Address      string `json:"address"`
	Nonce        int    `json:"nonce"`
	PendingNonce int    `json:"pendingNonce"`
}

func (m *nonceMessage) String() string {
	return fmt.Sprintf("%s:\nNonce: %d, Pending Nonce: %d",
		m.Address, m.Nonce, m.PendingNonce)
}
