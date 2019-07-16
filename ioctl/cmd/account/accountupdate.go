// Copyright (c) 2019 IoTeX
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

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// accountUpdateCmd represents the account update command
var accountUpdateCmd = &cobra.Command{
	Use:   "update [ALIAS|ADDRESS]",
	Short: "Update password for IoTeX account",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := accountUpdate(args)
		return err
	},
}

func accountUpdate(args []string) error {
	account, err := util.GetAddress(args)
	if err != nil {
		return output.PrintError(output.AddressError, err.Error())
	}
	address, err := address.FromString(account)
	if err != nil {
		return output.PrintError(output.ConvertError, "failed to convert string into address")
	}
	// find the keystore and update
	ks := keystore.NewKeyStore(config.ReadConfig.Wallet,
		keystore.StandardScryptN, keystore.StandardScryptP)
	for _, v := range ks.Accounts() {
		if bytes.Equal(address.Bytes(), v.Address.Bytes()) {
			fmt.Printf("#%s: Enter current password\n", account)
			currentPassword, err := util.ReadSecretFromStdin()
			if err != nil {
				return output.PrintError(output.InputError, "failed to get current password")
			}
			_, err = ks.SignHashWithPassphrase(v, currentPassword, hash.ZeroHash256[:])
			if err != nil {
				return output.PrintError(output.KeystoreError, err.Error())
			}
			fmt.Printf("#%s: Enter new password\n", account)
			password, err := util.ReadSecretFromStdin()
			if err != nil {
				return output.PrintError(output.InputError, "failed to get current password")
			}
			fmt.Printf("#%s: Enter new password again\n", account)
			passwordAgain, err := util.ReadSecretFromStdin()
			if err != nil {
				return output.PrintError(output.InputError, "failed to get current password")
			}
			if password != passwordAgain {
				return output.PrintError(output.InputError, ErrPasswdNotMatch.Error())
			}
			if err := ks.Update(v, currentPassword, password); err != nil {
				return output.PrintError(output.KeystoreError, "failed to update keystore")
			}
			output.PrintResult(fmt.Sprintf("Account #%s has been updated.", account))
		}
	}
	return output.PrintError(output.KeystoreError, fmt.Sprintf("account #%s not found", account))
}
