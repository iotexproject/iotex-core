// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
)

// Multi-language support
var (
	_hdwalletExportCmdShorts = map[config.Language]string{
		config.English: "export hdwallet mnemonic using password",
		config.Chinese: "通过密码导出钱包助记词",
	}
	_hdwalletExportCmdUses = map[config.Language]string{
		config.English: "export",
		config.Chinese: "export 导出",
	}
)

// NewHdwalletExportCmd represents the hdwallet export command
func NewHdwalletExportCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_hdwalletExportCmdUses)
	short, _ := client.SelectTranslation(_hdwalletExportCmdShorts)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			if !fileutil.FileExists(_hdWalletConfigFile) {
				cmd.Println("Run 'ioctl hdwallet create' to create your HDWallet first.")
				return nil
			}

			cmd.Println("Enter password")
			password, err := client.ReadSecret()
			if err != nil {
				return errors.New("failed to get password")
			}

			enctxt, err := os.ReadFile(_hdWalletConfigFile)
			if err != nil {
				return errors.New("failed to read config")
			}

			enckey := util.HashSHA256([]byte(password))
			dectxt, err := util.Decrypt(enctxt, enckey)
			if err != nil {
				return errors.New("failed to decrypt")
			}

			dectxtLen := len(dectxt)
			if dectxtLen <= 32 {
				return errors.New("incorrect data")
			}

			mnemonic, hash := dectxt[:dectxtLen-32], dectxt[dectxtLen-32:]
			if !bytes.Equal(hash, util.HashSHA256(mnemonic)) {
				return errors.New("password error")
			}

			cmd.Println(fmt.Sprintf("Mnemonic phrase: %s"+
				"It is used to recover your wallet in case you forgot the password. Write them down and store it in a safe place.", mnemonic))
			return nil
		},
	}
	return cmd
}
