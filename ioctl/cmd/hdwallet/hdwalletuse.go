// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"fmt"
	"io/ioutil"
	"strconv"

	"github.com/ethereum/go-ethereum/crypto"
	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"
	"github.com/spf13/cobra"

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
		config.English: "use ID1 ID2",
		config.Chinese: "use ID1 ID2",
	}
)

// hdwalletUseCmd represents the hdwallet use command
var hdwalletUseCmd = &cobra.Command{
	Use:   config.TranslateInLang(hdwalletUseCmdUses, config.UILanguage),
	Short: config.TranslateInLang(hdwalletUseCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		var arg [2]uint32
		for i := 0; i < 2; i++ {
			u64, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				return output.NewError(output.InputError, fmt.Sprintf("%v must be integer value", args[i]), err)
			}
			arg[i] = uint32(u64)
		}
		err := hdwalletUse(arg)
		return output.PrintError(err)
	},
}

func hdwalletUse(arg [2]uint32) error {
	if !fileutil.FileExists(hdWalletConfigFile) {
		output.PrintResult("No hdwallet created yet. please create first.")
		return nil
	}

	output.PrintQuery("Enter password\n")
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return output.NewError(output.InputError, "failed to get password", err)
	}

	content, err := ioutil.ReadFile(hdWalletConfigFile)
	if err != nil {
		return output.NewError(output.InputError, "failed to read config", err)
	}

	mnemonic, err := util.DecryptString(string(content), password)
	if err != nil {
		return output.NewError(output.InputError, "failed to decrypt", err)
	}

	wallet, err := hdwallet.NewFromMnemonic(mnemonic)
	if err != nil {
		return err
	}

	derivationPath := fmt.Sprintf("%s/%d/%d", DefaultRootDerivationPath[:len(DefaultRootDerivationPath)-2], arg[0], arg[1])

	path := hdwallet.MustParseDerivationPath(derivationPath)
	account, err := wallet.Derive(path, false)
	if err != nil {
		return err
	}

	private, err := wallet.PrivateKey(account)
	if err != nil {
		return err
	}
	addr, err := address.FromBytes(hashECDSAPublicKey(&private.PublicKey))
	if err != nil {
		return output.NewError(output.ConvertError, "failed to convert public key into address", err)
	}
	output.PrintResult(fmt.Sprintf("address: %s\nprivateKey: %x\npublicKey: %x",
		addr.String(), crypto.FromECDSA(private), crypto.FromECDSAPub(&private.PublicKey)))

	return nil
}
