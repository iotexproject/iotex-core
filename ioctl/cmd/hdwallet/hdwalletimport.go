// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tyler-smith/go-bip39"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	gohdwallet "github.com/miguelmota/go-ethereum-hdwallet"

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
	if fileutil.FileExists(hdWalletConfigDir) {
		output.PrintResult("already created hdwallet. please execute delete before import.")
		return nil
	}
	ks := keystore.NewKeyStore(hdWalletConfigDir,
		keystore.StandardScryptN, keystore.StandardScryptP)

	output.PrintQuery("Enter 12-words mnemonic:\n")

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

	output.PrintResult(fmt.Sprintf("Mnemonic pharse: %s\n"+
		"Save them somewhere safe and secret.", mnemonic))

	wallet, err := gohdwallet.NewFromMnemonic(mnemonic)
	if err != nil {
		return err
	}
	derivationPath := gohdwallet.MustParseDerivationPath(DefaultRootDerivationPath)
	account, err := wallet.Derive(derivationPath, false)
	if err != nil {
		return err
	}
	privateKey, err := wallet.PrivateKey(account)
	if err != nil {
		return err
	}

	_, err = ks.ImportECDSA(privateKey, password)

	return err
}
