// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"syscall"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
)

var (
	privateKey string
)

// accountImportCmd represents the account create command
var accountImportCmd = &cobra.Command{
	Use:   "import",
	Short: "import IoTeX private key into wallet",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(accountImport(args))
	},
}

func init() {
	accountImportCmd.Flags().StringVarP(&privateKey,
		"private-key", "k", "", "import account by private key")
}

func accountImport(args []string) string {
	name := args[0]
	cfg, err := config.LoadConfig()
	if err != nil {
		return err.Error()
	}
	if _, ok := cfg.AccountList[name]; ok {
		return fmt.Sprintf("A account named \"%s\" already exists.", name)
	}
	addr, err := newAccountByKey(name, privateKey)
	if err != nil {
		return err.Error()
	}
	cfg.AccountList[name] = addr
	out, err := yaml.Marshal(&cfg)
	if err != nil {
		return err.Error()
	}
	if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		return fmt.Sprintf("Failed to write to config file %s.", config.DefaultConfigFile)
	}
	return fmt.Sprintf(
		"New account \"%s\" is created. Keep your password, or your will lose your private key.",
		name,
	)
}

func newAccountByKey(name string, privateKey string) (string, error) {
	fmt.Printf("#%s: Enter password\n", name)
	bytePassword, err := terminal.ReadPassword(syscall.Stdin)
	if err != nil {
		log.L().Error("fail to get password", zap.Error(err))
		return "", err
	}
	password := strings.TrimSpace(string(bytePassword))
	fmt.Printf("#%s: Enter password again\n", name)
	bytePassword, err = terminal.ReadPassword(syscall.Stdin)
	if err != nil {
		log.L().Error("fail to get password", zap.Error(err))
		return "", err
	}
	if password != strings.TrimSpace(string(bytePassword)) {
		return "", errors.New("password doesn't match")
	}
	ks := keystore.NewKeyStore(config.ConfigDir, keystore.StandardScryptN, keystore.StandardScryptP)
	priKey, err := keypair.DecodePrivateKey(privateKey)
	if err != nil {
		return "", err
	}
	account, err := ks.ImportECDSA(priKey, password)
	if err != nil {
		return "", err
	}
	addr, _ := address.FromBytes(account.Address.Bytes())
	return addr.String(), nil
}
