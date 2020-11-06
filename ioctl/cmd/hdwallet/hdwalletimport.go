// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tyler-smith/go-bip39"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
)

// Multi-language support
var (
	importCmdShorts = map[config.Language]string{
		config.English: "import hdwallet using mnemonic",
		config.Chinese: "通过助记词导入钱包",
	}
	importCmdUses = map[config.Language]string{
		config.English: "import",
		config.Chinese: "import 导入",
	}
)

// hdwalletImportCmd represents the hdwallet import command
var hdwalletImportCmd = &cobra.Command{
	Use:   config.TranslateInLang(importCmdUses, config.UILanguage),
	Short: config.TranslateInLang(importCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := hdwalletImport()
		return output.PrintError(err)
	},
}

func hdwalletImport() error {
	if fileutil.FileExists(hdWalletConfigFile) {
		output.PrintResult("Please run 'ioctl hdwallet delete' before import")
		return nil
	}

	output.PrintQuery("Enter 12 mnemonic words you saved, separated by space\n")

	in := bufio.NewReader(os.Stdin)
	line, err := in.ReadString('\n')
	if err != nil {
		return err
	}
	mnemonic := strings.TrimSpace(line)
	if _, err = bip39.MnemonicToByteArray(mnemonic); err != nil {
		return err
	}

	output.PrintQuery("Set password\n")
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return output.NewError(output.InputError, "failed to get password", err)
	}
	output.PrintQuery("Enter password again\n")
	passwordAgain, err := util.ReadSecretFromStdin()
	if err != nil {
		return output.NewError(output.InputError, "failed to get password", err)
	}
	if password != passwordAgain {
		return output.NewError(output.ValidationError, ErrPasswdNotMatch.Error(), nil)
	}

	enctxt := append([]byte(mnemonic), util.HashSHA256([]byte(mnemonic))...)
	enckey := util.HashSHA256([]byte(password))
	out, err := util.Encrypt(enctxt, enckey)
	if err != nil {
		return output.NewError(output.ValidationError, "failed to encrypting mnemonic", nil)
	}

	if err := ioutil.WriteFile(hdWalletConfigFile, out, 0600); err != nil {
		return output.NewError(output.WriteFileError,
			fmt.Sprintf("failed to write to config file %s", hdWalletConfigFile), err)
	}

	output.PrintResult(fmt.Sprintf("Mnemonic pharse: %s\n"+
		"It is used to recover your wallet in case you forgot the password. Write them down and store it in a safe place.", mnemonic))

	return err
}
