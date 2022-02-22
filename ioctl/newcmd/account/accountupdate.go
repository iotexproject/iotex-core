// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"bytes"
	"fmt"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	updateCmdShorts = map[config.Language]string{
		config.English: "Update password for IoTeX account",
		config.Chinese: "为IoTeX账户更新密码",
	}
	updateCmdUses = map[config.Language]string{
		config.English: "update [ALIAS|ADDRESS]",
		config.Chinese: "update [别名|地址]",
	}
)

// NewAccountUpdate represents the account update command
func NewAccountUpdate(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(updateCmdShorts)
	short, _ := client.SelectTranslation(updateCmdUses)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			arg := ""
			if len(args) == 1 {
				arg = args[0]
			}

			acc, err := client.GetAddress(arg)
			if err != nil {
				return output.NewError(output.AddressError, "", err)
			}
			addr, err := address.FromString(acc)
			if err != nil {
				return output.NewError(output.ConvertError, "failed to convert string into addr", err)
			}

			if client.IsCryptoSm2() {
				// find the pem file and update
				filePath, err := findSm2PemFile(addr)
				if err != nil {
					return output.NewError(output.ReadFileError, fmt.Sprintf("crypto file of account #%s not found", addr), err)
				}
				if err = readSecretAndUpdate(client, acc,
					func(currentPassword string) error {
						_, err = crypto.ReadPrivateKeyFromPem(filePath, currentPassword)
						if err != nil {
							return output.NewError(output.CryptoError, "error occurs when checking current password", err)
						}
						return nil
					},
					func(currentPassword, newPassword string) error {
						if err := crypto.UpdatePrivateKeyPasswordToPem(filePath, currentPassword, newPassword); err != nil {
							return output.NewError(output.KeystoreError, "failed to update pem file", err)
						}
						return nil
					},
				); err != nil {
					return err
				}
			} else {
				// find the keystore and update
				ks := client.NewKeyStore()
				for _, v := range ks.Accounts() {
					if bytes.Equal(addr.Bytes(), v.Address.Bytes()) {
						if err = readSecretAndUpdate(client, acc,
							func(currentPassword string) error {
								_, err = ks.SignHashWithPassphrase(v, currentPassword, hash.ZeroHash256[:])
								if err != nil {
									return output.NewError(output.KeystoreError, "error occurs when checking current password", err)
								}
								return nil
							},
							func(currentPassword, newPassword string) error {
								if err := ks.Update(v, currentPassword, newPassword); err != nil {
									return output.NewError(output.KeystoreError, "failed to update keystore", err)
								}
								return nil
							},
						); err != nil {
							return err
						}
					}
				}
			}
			output.PrintResult(fmt.Sprintf("Account #%s has been updated.", acc))
			return nil
		},
	}
	return cmd
}

func readSecretAndUpdate(client ioctl.Client, acc string,
	checkPwdFunc func(currentPassword string) error,
	updatePwdFunc func(currentPassword, newPassword string) error) error {
	output.PrintQuery(fmt.Sprintf("#%s: Enter current password\n", acc))
	currentPassword, err := client.ReadSecret()
	if err != nil {
		return output.NewError(output.InputError, "failed to get current password", err)
	}
	if err = checkPwdFunc(currentPassword); err != nil {
		return err
	}
	output.PrintQuery(fmt.Sprintf("#%s: Enter new password\n", acc))
	newPassword, err := client.ReadSecret()
	if err != nil {
		return output.NewError(output.InputError, "failed to get new password", err)
	}
	output.PrintQuery(fmt.Sprintf("#%s: Enter new password again\n", acc))
	newPasswordAgain, err := client.ReadSecret()
	if err != nil {
		return output.NewError(output.InputError, "failed to get new password", err)
	}
	if newPassword != newPasswordAgain {
		return output.NewError(output.ValidationError, ErrPasswdNotMatch.Error(), nil)
	}
	return updatePwdFunc(currentPassword, newPassword)
}
