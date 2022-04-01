// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/spf13/cobra"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	_updateCmdShorts = map[config.Language]string{
		config.English: "Update password for IoTeX account",
		config.Chinese: "为IoTeX账户更新密码",
	}
	_updateCmdUses = map[config.Language]string{
		config.English: "update [ALIAS|ADDRESS]",
		config.Chinese: "update [别名|地址]",
	}
)

// _accountUpdateCmd represents the account update command
var _accountUpdateCmd = &cobra.Command{
	Use:   config.TranslateInLang(_updateCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_updateCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		arg := ""
		if len(args) == 1 {
			arg = args[0]
		}
		err := accountUpdate(arg)
		return output.PrintError(err)
	},
}

func accountUpdate(arg string) error {
	account, err := util.GetAddress(arg)
	if err != nil {
		return output.NewError(output.AddressError, "", err)
	}
	addr, err := address.FromString(account)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to convert string into addr", err)
	}

	if CryptoSm2 {
		// find the pem file and update
		filePath, err := findSm2PemFile(addr)
		if err != nil {
			return output.NewError(output.ReadFileError, fmt.Sprintf("crypto file of account #%s not found", addr), err)
		}

		output.PrintQuery(fmt.Sprintf("#%s: Enter current password\n", account))
		currentPassword, err := util.ReadSecretFromStdin()
		if err != nil {
			return output.NewError(output.InputError, "failed to get current password", err)
		}
		_, err = crypto.ReadPrivateKeyFromPem(filePath, currentPassword)
		if err != nil {
			return output.NewError(output.CryptoError, "error occurs when checking current password", err)
		}
		output.PrintQuery(fmt.Sprintf("#%s: Enter new password\n", account))
		password, err := util.ReadSecretFromStdin()
		if err != nil {
			return output.NewError(output.InputError, "failed to get new password", err)
		}
		output.PrintQuery(fmt.Sprintf("#%s: Enter new password again\n", account))
		passwordAgain, err := util.ReadSecretFromStdin()
		if err != nil {
			return output.NewError(output.InputError, "failed to get new password", err)
		}
		if password != passwordAgain {
			return output.NewError(output.ValidationError, ErrPasswdNotMatch.Error(), nil)
		}

		if err := crypto.UpdatePrivateKeyPasswordToPem(filePath, currentPassword, password); err != nil {
			return output.NewError(output.KeystoreError, "failed to update pem file", err)
		}
	} else {
		// find the keystore and update
		ks := keystore.NewKeyStore(config.ReadConfig.Wallet,
			keystore.StandardScryptN, keystore.StandardScryptP)
		for _, v := range ks.Accounts() {
			if bytes.Equal(addr.Bytes(), v.Address.Bytes()) {
				output.PrintQuery(fmt.Sprintf("#%s: Enter current password\n", account))
				currentPassword, err := util.ReadSecretFromStdin()
				if err != nil {
					return output.NewError(output.InputError, "failed to get current password", err)
				}
				_, err = ks.SignHashWithPassphrase(v, currentPassword, hash.ZeroHash256[:])
				if err != nil {
					return output.NewError(output.KeystoreError, "error occurs when checking current password", err)
				}
				output.PrintQuery(fmt.Sprintf("#%s: Enter new password\n", account))
				password, err := util.ReadSecretFromStdin()
				if err != nil {
					return output.NewError(output.InputError, "failed to get new password", err)
				}
				output.PrintQuery(fmt.Sprintf("#%s: Enter new password again\n", account))
				passwordAgain, err := util.ReadSecretFromStdin()
				if err != nil {
					return output.NewError(output.InputError, "failed to get new password", err)
				}
				if password != passwordAgain {
					return output.NewError(output.ValidationError, ErrPasswdNotMatch.Error(), nil)
				}

				if err := ks.Update(v, currentPassword, password); err != nil {
					return output.NewError(output.KeystoreError, "failed to update keystore", err)
				}

				output.PrintResult(fmt.Sprintf("Account #%s has been updated.", account))
				return nil
			}
		}
	}

	output.PrintResult(fmt.Sprintf("Account #%s has been updated.", account))
	return nil
}
