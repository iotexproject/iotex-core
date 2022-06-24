// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"fmt"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/tyler-smith/go-bip39"
)

// Multi-language support
var (
	_createByMnemonicCmdShorts = map[config.Language]string{
		config.English: "create hdwallet using mnemonic",
		config.Chinese: "通过助记词创建新钱包",
	}
	_createByMnemonicCmdUses = map[config.Language]string{
		config.English: "create",
		config.Chinese: "create 创建",
	}
)

// NewHdwalletCreateCmd represents the hdwallet create command
func NewHdwalletCreateCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_createByMnemonicCmdUses)
	short, _ := client.SelectTranslation(_createByMnemonicCmdShorts)

	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			if client.IsHdWalletConfigFileExist() {
				cmd.Println("Please run 'ioctl hdwallet delete' before import")
				return nil
			}
			cmd.Println("Set password")
			password, err := client.ReadSecret()
			if err != nil {
				return errors.Wrap(err, "failed to get password")
			}
			cmd.Println("Enter password again")
			passwordAgain, err := client.ReadSecret()
			if err != nil {
				return errors.Wrap(err, "failed to get password")
			}
			if password != passwordAgain {
				return errors.New(ErrPasswdNotMatch.Error())
			}

			entropy, _ := bip39.NewEntropy(128)
			mnemonic, _ := bip39.NewMnemonic(entropy)

			if err = client.WriteHdWalletConfigFile(password, mnemonic); err != nil {
				return err
			}

			cmd.Println(fmt.Sprintf("Mnemonic phrase: %s\n"+
				"It is used to recover your wallet in case you forgot the password. Write them down and store it in a safe place.", mnemonic))
			return nil
		},
	}
}
