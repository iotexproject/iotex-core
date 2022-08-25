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

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	_createShorts = map[config.Language]string{
		config.English: "Create N new accounts and print them",
		config.Chinese: "创建 N 个新账户，并打印",
	}
	_createFlagUsages = map[config.Language]string{
		config.English: "number of accounts to create",
		config.Chinese: "指定创建账户的数量",
	}
	_failToGenerateNewPrivateKey = map[config.Language]string{
		config.English: "failed to generate new private key",
		config.Chinese: "生成新私钥失败",
	}
	_failToGenerateNewPrivateKeySm2 = map[config.Language]string{
		config.English: "failed to generate new sm2 private key",
		config.Chinese: "生成新sm2私钥失败",
	}
	_failToConvertPublicKeyIntoAddress = map[config.Language]string{
		config.English: "failed to convert public key into address",
		config.Chinese: "将公钥转换为地址失败",
	}
)

// NewAccountCreate represents the account create command
func NewAccountCreate(client ioctl.Client) *cobra.Command {
	var numAccounts uint
	short, _ := client.SelectTranslation(_createShorts)
	usage, _ := client.SelectTranslation(_createFlagUsages)
	_failToGenerateNewPrivateKey, _ := client.SelectTranslation(_failToGenerateNewPrivateKey)
	_failToGenerateNewPrivateKeySm2, _ := client.SelectTranslation(_failToGenerateNewPrivateKeySm2)
	_failToConvertPublicKeyIntoAddress, _ := client.SelectTranslation(_failToConvertPublicKeyIntoAddress)

	cmd := &cobra.Command{
		Use:   "create",
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			var err error
			var private crypto.PrivateKey

			newAccounts := make([]generatedAccount, 0)
			for i := 0; i < int(numAccounts); i++ {
				if !client.IsCryptoSm2() {
					private, err = crypto.GenerateKey()
					if err != nil {
						return errors.Wrap(err, _failToGenerateNewPrivateKey)
					}
				} else {
					private, err = crypto.GenerateKeySm2()
					if err != nil {
						return errors.Wrap(err, _failToGenerateNewPrivateKeySm2)
					}
				}

				addr := private.PublicKey().Address()
				if addr == nil {
					return errors.New(_failToConvertPublicKeyIntoAddress)
				}
				newAccount := generatedAccount{
					Address:    addr.String(),
					PrivateKey: fmt.Sprintf("%x", private.Bytes()),
					PublicKey:  fmt.Sprintf("%x", private.PublicKey().Bytes()),
				}
				newAccounts = append(newAccounts, newAccount)
			}

			message := createMessage{Accounts: newAccounts}
			cmd.Println(message.String())
			return nil
		},
	}
	cmd.Flags().UintVarP(&numAccounts, "num", "n", 1, usage)
	return cmd
}

type createMessage struct {
	Accounts []generatedAccount `json:"accounts"`
}

type generatedAccount struct {
	Address    string `json:"address"`
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
}

func (m *createMessage) String() string {
	byteAsJSON, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		log.Panic(err)
	}
	return fmt.Sprint(string(byteAsJSON))
}
