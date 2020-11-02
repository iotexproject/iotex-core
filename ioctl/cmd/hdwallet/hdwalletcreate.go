// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tyler-smith/go-bip39"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	gohdwallet "github.com/miguelmota/go-ethereum-hdwallet"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	createByMnemonicCmdShorts = map[config.Language]string{
		config.English: "create hdwallet using mnemonic",
		config.Chinese: "通过助记词创建新钱包",
	}
	createByMnemonicCmdUses = map[config.Language]string{
		config.English: "create",
		config.Chinese: "create 创建",
	}
)

// hdwalletCreateCmd represents the hdwallet create command
var hdwalletCreateCmd = &cobra.Command{
	Use:   config.TranslateInLang(createByMnemonicCmdUses, config.UILanguage),
	Short: config.TranslateInLang(createByMnemonicCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := hdwalletCreate()
		return output.PrintError(err)
	},
}

func hdwalletCreate() error {

	ks := keystore.NewKeyStore(hdWalletConfigDir,
		keystore.StandardScryptN, keystore.StandardScryptP)

	if len(ks.Accounts()) > 0 {
		output.PrintResult("already created hdwallet, if you forgot password，use delete/import command.")
		return nil
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

	entropy, _ := bip39.NewEntropy(128)
	mnemonic, _ := bip39.NewMnemonic(entropy)

	output.PrintResult(fmt.Sprintf("Mnemonic pharse: %s\n"+
		"Save them somewhere safe and secret.", mnemonic))

	wallet, err := gohdwallet.NewFromMnemonic(string(mnemonic))
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
