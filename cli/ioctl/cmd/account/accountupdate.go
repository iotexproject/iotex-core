// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"bytes"
	"fmt"
	"syscall"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// accountUpdateCmd represents the account update command
var accountUpdateCmd = &cobra.Command{
	Use:   "update (ALIAS|ADDRESS)",
	Short: "Update password for IoTeX account",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(accountUpdate(args))
	},
}

func accountUpdate(args []string) string {
	account := args[0]
	addr, err := alias.Address(account)
	if err != nil {
		return err.Error()
	}
	address, err := address.FromString(addr)
	if err != nil {
		log.L().Error("failed to convert string into address", zap.Error(err))
		return err.Error()
	}
	// find the keystore and update
	ks := keystore.NewKeyStore(config.ReadConfig.Wallet,
		keystore.StandardScryptN, keystore.StandardScryptP)
	for _, v := range ks.Accounts() {
		if bytes.Equal(address.Bytes(), v.Address.Bytes()) {
			fmt.Printf("#%s: Enter current password\n", account)
			byteCurrentPassword, err := terminal.ReadPassword(int(syscall.Stdin))
			if err != nil {
				log.L().Error("failed to get current password", zap.Error(err))
				return err.Error()
			}
			currentPassword := string(byteCurrentPassword)
			_, err = ks.SignHashWithPassphrase(v, currentPassword, hash.ZeroHash256[:])
			if err != nil {
				return "wrong password"
			}
			fmt.Printf("#%s: Enter new password\n", account)
			bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
			if err != nil {
				log.L().Error("failed to get password", zap.Error(err))
				return err.Error()
			}
			password := string(bytePassword)
			fmt.Printf("#%s: Enter new password again\n", account)
			bytePassword, err = terminal.ReadPassword(int(syscall.Stdin))
			if err != nil {
				log.L().Error("failed to get password", zap.Error(err))
				return err.Error()
			}
			if password != string(bytePassword) {
				return ErrPasswdNotMatch.Error()
			}
			if err := ks.Update(v, currentPassword, password); err != nil {
				log.L().Error("failed to update keystore", zap.Error(err))
				return err.Error()
			}
			return fmt.Sprintf("Account #%s has been updated.", account)
		}
	}
	return fmt.Sprintf("Account #%s not found", account)
}
