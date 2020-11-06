// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"bytes"
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
)

// Multi-language support
var (
	hdwalletExportCmdShorts = map[config.Language]string{
		config.English: "export hdwallet mnemonic using password",
		config.Chinese: "通过密码导出钱包助记词",
	}
	hdwalletExportCmdUses = map[config.Language]string{
		config.English: "export",
		config.Chinese: "export 导出",
	}
)

// hdwalletExportCmd represents the hdwallet export command
var hdwalletExportCmd = &cobra.Command{
	Use:   config.TranslateInLang(hdwalletExportCmdUses, config.UILanguage),
	Short: config.TranslateInLang(hdwalletExportCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := hdwalletExport()
		return output.PrintError(err)
	},
}

func hdwalletExport() error {
	if !fileutil.FileExists(hdWalletConfigFile) {
		output.PrintResult("Run 'ioctl hdwallet create' to create your HDWallet first.")
		return nil
	}

	output.PrintQuery("Enter password\n")
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return output.NewError(output.InputError, "failed to get password", err)
	}

	enctxt, err := ioutil.ReadFile(hdWalletConfigFile)
	if err != nil {
		return output.NewError(output.InputError, "failed to read config", err)
	}

	enckey := util.HashSHA256([]byte(password))

	dectxt, err := util.Decrypt(enctxt, enckey)
	if err != nil {
		return output.NewError(output.InputError, "failed to decrypt", err)
	}

	dectxtLen := len(dectxt)
	if dectxtLen <= 32 {
		return fmt.Errorf("incorrect data")
	}

	mnemonic, hash := dectxt[:dectxtLen-32], dectxt[dectxtLen-32:]

	if !bytes.Equal(hash, util.HashSHA256(mnemonic)) {
		return fmt.Errorf("password error")
	}

	output.PrintResult(fmt.Sprintf("Mnemonic pharse: %s\n"+
		"It is used to recover your wallet in case you forgot the password. Write them down and store it in a safe place.", mnemonic))

	return nil
}
