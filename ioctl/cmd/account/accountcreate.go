// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

var numAccounts uint

// Multi-language support
var (
	_createCmdShorts = map[config.Language]string{
		config.English: "Create N new accounts and print them",
		config.Chinese: "创建 N 个新账户，并打印",
	}
	_createCmdUses = map[config.Language]string{
		config.English: "create",
		config.Chinese: "create 创建",
	}
	_flagNumUsages = map[config.Language]string{
		config.English: "number of accounts to create",
		config.Chinese: "指定创建账户的数量",
	}
)

// _accountCreateCmd represents the account create command
var _accountCreateCmd = &cobra.Command{
	Use:   config.TranslateInLang(_createCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_createCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := accountCreate()
		return output.PrintError(err)
	},
}

type createMessage struct {
	Accounts []generatedAccount `json:"accounts"`
}

type generatedAccount struct {
	Address    string `json:"address"`
	EthAddress string `json:"ethAddress"`
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
}

func init() {
	_accountCreateCmd.Flags().UintVarP(&numAccounts, "num", "n", 1,
		config.TranslateInLang(_flagNumUsages, config.UILanguage))
}

func accountCreate() error {
	var err error
	var private crypto.PrivateKey
	newAccounts := make([]generatedAccount, 0)
	for i := 0; i < int(numAccounts); i++ {
		if !CryptoSm2 {
			private, err = crypto.GenerateKey()
			if err != nil {
				return output.NewError(output.CryptoError, "failed to generate new private key", err)
			}

		} else {
			private, err = crypto.GenerateKeySm2()
			if err != nil {
				return output.NewError(output.CryptoError, "failed to generate new sm2 private key", err)
			}
		}
		addr := private.PublicKey().Address()
		if addr == nil {
			return output.NewError(output.ConvertError, "failed to convert public key into address", nil)
		}
		newAccount := generatedAccount{
			Address:    addr.String(),
			EthAddress: addr.Hex(),
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
