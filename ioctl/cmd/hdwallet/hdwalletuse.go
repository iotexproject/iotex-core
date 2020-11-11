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
	"strconv"

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
		return output.NewError(output.ValidationError, "incorrect data", nil)
	}

	mnemonic, hash := dectxt[:dectxtLen-32], dectxt[dectxtLen-32:]
	if !bytes.Equal(hash, util.HashSHA256(mnemonic)) {
		return output.NewError(output.ValidationError, "password error", nil)
	}

	wallet, err := hdwallet.NewFromMnemonic(string(mnemonic))
	if err != nil {
		return err
	}

	derivationPath := fmt.Sprintf("%s/0'/%d/%d", DefaultRootDerivationPath, arg[0], arg[1])
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
	output.PrintResult(fmt.Sprintf("address: %s\n", addr.String()))

	return nil
}
