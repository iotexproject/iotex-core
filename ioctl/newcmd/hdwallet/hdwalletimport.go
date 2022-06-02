// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/tyler-smith/go-bip39"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
)

// Multi-language support
var (
	_importCmdShorts = map[config.Language]string{
		config.English: "import hdwallet using mnemonic",
		config.Chinese: "通过助记词导入钱包",
	}
	_importCmdUses = map[config.Language]string{
		config.English: "import",
		config.Chinese: "import 导入",
	}
)

// NewHdwalletImportCmd represents the hdwallet import command
func NewHdwalletImportCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_importCmdUses)
	short, _ := client.SelectTranslation(_importCmdShorts)

	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			_hdWalletConfigFile = client.Config().Wallet + "/hdwallet"
			if fileutil.FileExists(_hdWalletConfigFile) {
				cmd.Println("Please run 'ioctl hdwallet delete' before import")
				return nil
			}

			cmd.Println("Enter 12 mnemonic words you saved, separated by space")

			line, err := client.ReadInput()
			if err != nil {
				return err
			}
			mnemonic := strings.TrimSpace(line)
			if _, err = bip39.MnemonicToByteArray(mnemonic); err != nil {
				return err
			}

			cmd.Println("Set password")
			password, err := client.ReadSecret()
			if err != nil {
				return errors.New("failed to get password")
			}
			cmd.Println("Enter password again")
			passwordAgain, err := client.ReadSecret()
			if err != nil {
				return errors.New("failed to get password")
			}
			if password != passwordAgain {
				return ErrPasswdNotMatch
			}

			enctxt := append([]byte(mnemonic), util.HashSHA256([]byte(mnemonic))...)
			enckey := util.HashSHA256([]byte(password))
			out, err := util.Encrypt(enctxt, enckey)
			if err != nil {
				return errors.Wrap(err, "failed to encrypting mnemonic")
			}

			if err := client.WriteFile(_hdWalletConfigFile, out); err != nil {
				return errors.Wrap(err, "failed to write to config file")
			}

			cmd.Printf("Mnemonic phrase: %s\n"+
				"It is used to recover your wallet in case you forgot the password. Write them down and store it in a safe place.", mnemonic)
			return nil
		},
	}
}
