// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/spf13/cobra"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
)

// Multi-language support
var (
	hdwalletUseCmdShorts = map[config.Language]string{
		config.English: "create hdwallet account",
		config.Chinese: "生成钱包账号",
	}
	hdwalletUseCmdUses = map[config.Language]string{
		config.English: "use",
		config.Chinese: "use 使用",
	}
)

// hdwalletUseCmd represents the hdwallet use command
var hdwalletUseCmd = &cobra.Command{
	Use:   config.TranslateInLang(hdwalletUseCmdUses, config.UILanguage),
	Short: config.TranslateInLang(hdwalletUseCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := hdwalletUse()
		return output.PrintError(err)
	},
}

func hdwalletUse() error {
	if !fileutil.FileExists(hdWalletConfigDir) {
		output.PrintResult("No hdwallet created yet. please create first.")
		return nil
	}

	output.PrintQuery("Enter password\n")
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return output.NewError(output.InputError, "failed to get password", err)
	}

	ks := keystore.NewKeyStore(hdWalletConfigDir,
		keystore.StandardScryptN, keystore.StandardScryptP)
	accounts := ks.Accounts()
	if len(accounts) == 0 {
		output.PrintResult("something wrong with keystore.")
		return nil
	}

	private, err := crypto.KeystoreToPrivateKey(accounts[0], password)
	if err != nil {
		return err
	}
	addr, err := address.FromBytes(private.PublicKey().Hash())
	if err != nil {
		return err
	}
	output.PrintResult(fmt.Sprintf("address: %s\nprivateKey: %x\npublicKey: %x",
		addr.String(), private.Bytes(), private.PublicKey().Bytes()))
	return nil
}
