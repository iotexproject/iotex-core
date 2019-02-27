// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package wallet

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
)

var (
	name, password string
)

// walletCreateCmd represents the wallet create command
var walletCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create new wallet for ioctl",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(walletCreate())
	},
}

func init() {
	walletCreateCmd.Flags().StringVarP(&name, "name", "n", "", "name for wallet")
	walletCreateCmd.Flags().StringVarP(&password, "password", "p", "", "password for wallet")
	walletCreateCmd.MarkFlagRequired("name")
	walletCreateCmd.MarkFlagRequired("password")
}

func walletCreate() string {
	success := "New wallet \"" + name + "\" created, password = " + password +
		"\n**Remember to save your password. The wallet will be lost if you forgot the password!!"
	file, err := ioutil.ReadFile(config.DefaultConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			// special case of empty config file just being created
			addr, err := newWallet()
			if err != nil {
				return err.Error()
			}
			line := walletPrefix + "\n" + name + ":" + addr + "\n" + walletEnd
			if err := ioutil.WriteFile(config.DefaultConfigFile, []byte(line), 0600); err != nil {
				return fmt.Sprintf("failed to create config file %s", config.DefaultConfigFile)
			}
			return success
		}
		return fmt.Sprintf("failed to open config file %s", config.DefaultConfigFile)
	}
	// parse the wallet section from config file
	_, end, find, exist := parseConfig(file, walletPrefix, walletEnd, name)
	if exist {
		return "A wallet named " + name + " already exists"
	}
	addr, err := newWallet()
	if err != nil {
		return err.Error()
	}
	// insert a line for the new wallet
	lines := strings.Split(string(file), "\n")
	if !find {
		lines = append(lines, walletPrefix, name+":"+addr, walletEnd)
	} else {
		after := make([]string, len(lines)-end)
		copy(after, lines[end:])
		lines[end] = name + ":" + addr
		lines = append(lines[:end+1], after...)
	}
	output := strings.Join(lines, "\n")
	if err := ioutil.WriteFile(config.DefaultConfigFile, []byte(output), 0644); err != nil {
		return fmt.Sprintf("failed to write to config file %s", config.DefaultConfigFile)
	}
	return success
}

func newWallet() (string, error) {
	ks := keystore.NewKeyStore(config.ConfigDir, keystore.StandardScryptN, keystore.StandardScryptP)
	account, err := ks.NewAccount(password)
	if err != nil {
		return "", err
	}
	addr, _ := address.FromBytes(account.Address.Bytes())
	return addr.String(), nil
}
