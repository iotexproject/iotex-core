// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	_hdwalletExportCmdShorts = map[config.Language]string{
		config.English: "export hdwallet mnemonic using password",
		config.Chinese: "通过密码导出钱包助记词",
	}
)

// NewHdwalletExportCmd represents the hdwallet export command
func NewHdwalletExportCmd(client ioctl.Client) *cobra.Command {
	short, _ := client.SelectTranslation(_hdwalletExportCmdShorts)

	return &cobra.Command{
		Use:   "export",
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			cmd.Println("Enter password")
			password, err := client.ReadSecret()
			if err != nil {
				return errors.Wrap(err, "failed to get password")
			}

			mnemonic, err := client.HdwalletMnemonic(password)
			if err != nil {
				return errors.Wrap(err, "failed to export mnemonic")
			}
			cmd.Printf("Mnemonic phrase: %s"+
				"It is used to recover your wallet in case you forgot the password. Write them down and store it in a safe place.\n", mnemonic)
			return nil
		},
	}
}
