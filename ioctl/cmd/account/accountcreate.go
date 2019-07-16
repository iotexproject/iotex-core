// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/output"
)

var numAccounts uint

// accountCreateCmd represents the account create command
var accountCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create N new accounts and print them",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := accountCreate()
		return err
	},
}

type createMessage struct {
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

func accountCreate() error {
	newAccounts := make([]generatedAccount, 0)
	for i := 0; i < int(numAccounts); i++ {
		private, err := crypto.GenerateKey()
		if err != nil {
			return output.PrintError(output.CryptoError, err.Error())
		}
		addr, err := address.FromBytes(private.PublicKey().Hash())
		if err != nil {
			return output.PrintError(output.ConvertError, err.Error())
		}
		newAccount := generatedAccount{
			Address:    addr.String(),
			PrivateKey: fmt.Sprintf("%x", private.Bytes()),
			PublicKey:  fmt.Sprintf("%x", private.PublicKey().Bytes()),
		}
		newAccounts = append(newAccounts, newAccount)
	}

	message := createMessage{Accounts: newAccounts}
	fmt.Println(message.String())
	return nil
}

func (m *createMessage) String() string {
	if output.Format == "" {
		byteAsJSON, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			log.Panic(err)
		}
		return fmt.Sprint(string(byteAsJSON))
	}
	return output.FormatString(output.Result, m)
}
