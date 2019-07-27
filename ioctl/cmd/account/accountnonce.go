// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// accountNonceCmd represents the account nonce command
var accountNonceCmd = &cobra.Command{
	Use:   "nonce [ALIAS|ADDRESS]",
	Short: "Get nonce of an account",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		arg := ""
		if len(args) == 1 {
			arg = args[0]
		}
		err := nonce(arg)
		return output.PrintError(err)
	},
}

type nonceMessage struct {
	Address      string `json:"address"`
	Nonce        int    `json:"nonce"`
	PendingNonce int    `json:"pendingNonce"`
}

// nonce gets nonce and pending nonce of an IoTeX blockchain address
func nonce(arg string) error {
	addr, err := util.GetAddress(arg)
	if err != nil {
		return output.NewError(output.AddressError, "failed to get address", err)
	}
	accountMeta, err := GetAccountMeta(addr)
	if err != nil {
		return output.NewError(0, "", err)
	}
	message := nonceMessage{
		Address:      addr,
		Nonce:        int(accountMeta.Nonce),
		PendingNonce: int(accountMeta.PendingNonce),
	}
	fmt.Println(message.String())
	return nil
}

func (m *nonceMessage) String() string {
	if output.Format == "" {
		return fmt.Sprintf("%s:\nNonce: %d, Pending Nonce: %d",
			m.Address, m.Nonce, m.PendingNonce)
	}
	return output.FormatString(output.Result, m)
}
