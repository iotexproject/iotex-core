// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package wallet

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
)

const (
	walletPrefix = "wallet:"
	walletEnd    = "endWallet"
)

// WalletCmd represents the wallet command
var WalletCmd = &cobra.Command{
	Use:   "wallet",
	Short: "Manage accounts",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Print: " + strings.Join(args, " "))
	},
}

func init() {
	WalletCmd.AddCommand(walletCreateCmd)
	WalletCmd.AddCommand(walletListCmd)
}

// Sign use the password to unlock key associated with name, and signs the hash
func Sign(name, password string, hash []byte) ([]byte, error) {
	w, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	addrStr, ok := w.WalletList[name]
	if !ok {
		return nil, errors.Errorf("wallet %s does not exist", name)
	}
	addr, err := address.FromString(addrStr)
	if err != nil {
		return nil, err
	}
	// find the key in keystore and sign
	ks := keystore.NewKeyStore(config.ConfigDir, keystore.StandardScryptN, keystore.StandardScryptP)
	for _, v := range ks.Accounts() {
		if bytes.Equal(addr.Bytes(), v.Address.Bytes()) {
			return ks.SignHashWithPassphrase(v, password, hash)
		}
	}
	return nil, errors.Errorf("wallet %s's address does not match with keys in keystore", name)
}
