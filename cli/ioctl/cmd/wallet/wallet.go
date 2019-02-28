// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package wallet

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"gopkg.in/yaml.v2"

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

type wallets struct {
	WalletList map[string]string `yaml:"walletList"`
}

func init() {
	WalletCmd.AddCommand(walletCreateCmd)
	WalletCmd.AddCommand(walletListCmd)
}

func parseConfig(file []byte, start, end, name string) (int, int, bool, bool) {
	var startLine, endLine int
	find := false
	exist := false
	lines := strings.Split(string(file), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, end) {
			endLine = i
			break
		}
		if !find && strings.HasPrefix(line, start) {
			find = true
			startLine = i
			continue
		}
		// detect name collision
		if find && name != "" && strings.HasPrefix(line, name) {
			exist = true
		}
	}
	return startLine, endLine, find, exist
}

func loadWallets() (wallets, error) {
	w := wallets{
		WalletList: make(map[string]string),
	}
	in, err := ioutil.ReadFile(config.DefaultConfigFile)
	if err == nil {
		if err := yaml.Unmarshal(in, &w); err != nil {
			return w, err
		}
	} else if !os.IsNotExist(err) {
		return w, err
	}
	return w, nil
}

// Sign use the password to unlock key associated with name, and signs the hash
func Sign(name, password string, hash []byte) ([]byte, error) {
	w, err := loadWallets()
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
