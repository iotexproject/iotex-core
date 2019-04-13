// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
)

var numAccounts uint

// accountCreateCmd represents the account create command
var accountCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create N new accounts and print them",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := accountCreate()
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

type generatedAccounts struct {
	Accounts []generatedAccount `json:"accounts"`
}

type generatedAccount struct {
	Address    string `json:"address"`
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
}

func init() {
	accountCreateCmd.Flags().UintVarP(&numAccounts, "num", "n", 1, "number of accounts to create")
}

func accountCreate() (string, error) {
	newAccounts := make([]generatedAccount, 0)
	for i := 0; i < int(numAccounts); i++ {
		private, err := keypair.GenerateKey()
		if err != nil {
			return "", err
		}
		addr, err := address.FromBytes(private.PublicKey().Hash())
		if err != nil {
			log.L().Error("failed to convert bytes into address", zap.Error(err))
			return "", err
		}
		newAccount := generatedAccount{
			Address:    addr.String(),
			PrivateKey: fmt.Sprintf("%x", private.Bytes()),
			PublicKey:  fmt.Sprintf("%x", private.PublicKey().Bytes()),
		}
		newAccounts = append(newAccounts, newAccount)
	}
	var output []byte
	var err error
	output, err = json.MarshalIndent(&generatedAccounts{Accounts: newAccounts}, "", "  ")
	if err != nil {
		return "", err
	}
	return string(output), nil
}
