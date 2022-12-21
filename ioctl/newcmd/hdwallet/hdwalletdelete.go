// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	_hdwalletDeleteCmdShorts = map[config.Language]string{
		config.English: "delete hdwallet",
		config.Chinese: "删除钱包",
	}
)

// NewHdwalletDeleteCmd represents the hdwallet delete command
func NewHdwalletDeleteCmd(client ioctl.Client) *cobra.Command {
	short, _ := client.SelectTranslation(_hdwalletDeleteCmdShorts)

	return &cobra.Command{
		Use:   "delete",
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			info := fmt.Sprintf("** This is an irreversible action!\n" +
				"Once an hdwallet is deleted, all the assets under this hdwallet may be lost!\n" +
				"Type 'YES' to continue, quit for anything else.")
			confirmed, err := client.AskToConfirm(info)
			if err != nil {
				return errors.Wrap(err, "failed to ask confirm")
			}
			if !confirmed {
				cmd.Println("quit")
				return nil
			}

			return client.RemoveHdWalletConfigFile()
		},
	}
}
