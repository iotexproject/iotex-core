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
)

// Multi-language support
var (
	createShorts = map[ioctl.Language]string{
		ioctl.English: "Create N new accounts and print them",
		ioctl.Chinese: "创建 N 个新账户，并打印",
	}
	createUses = map[ioctl.Language]string{
		ioctl.English: "create",
		ioctl.Chinese: "create 创建",
	}
	createFlagUsages = map[ioctl.Language]string{
		ioctl.English: "number of accounts to create",
		ioctl.Chinese: "指定创建账户的数量",
	}
	failToGenerateNewPrivateKey = map[ioctl.Language]string{
		ioctl.English: "failed to generate new private key",
		ioctl.Chinese: "生成新私钥失败",
	}
	failToGenerateNewPrivateKeySm2 = map[ioctl.Language]string{
		ioctl.English: "failed to generate new sm2 private key",
		ioctl.Chinese: "生成新sm2私钥失败",
	}
	failToConvertPublicKeyIntoAddress = map[ioctl.Language]string{
		ioctl.English: "failed to convert public key into address",
		ioctl.Chinese: "将公钥转换为地址失败",
	}
)

// NewAccountCreate represents the account create command
func NewAccountCreate(client ioctl.Client) *cobra.Command {
	var numAccounts uint
	use, _ := client.SelectTranslation(createUses)
	short, _ := client.SelectTranslation(createShorts)
	usage, _ := client.SelectTranslation(createFlagUsages)
	failToGenerateNewPrivateKey, _ := client.SelectTranslation(failToGenerateNewPrivateKey)
	failToGenerateNewPrivateKeySm2, _ := client.SelectTranslation(failToGenerateNewPrivateKeySm2)
	failToConvertPublicKeyIntoAddress, _ := client.SelectTranslation(failToConvertPublicKeyIntoAddress)

	cmd := &cobra.Command{
		Use:   use,
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
						return errors.Wrap(err, failToGenerateNewPrivateKey)
					}
				} else {
					private, err = crypto.GenerateKeySm2()
					if err != nil {
						return errors.Wrap(err, failToGenerateNewPrivateKeySm2)
					}
				}

				addr := private.PublicKey().Address()
				if addr == nil {
					return errors.New(failToConvertPublicKeyIntoAddress)
				}
				newAccount := generatedAccount{
					Address:    addr.String(),
					PrivateKey: fmt.Sprintf("%x", private.Bytes()),
					PublicKey:  fmt.Sprintf("%x", private.PublicKey().Bytes()),
				}
				newAccounts = append(newAccounts, newAccount)
			}

			message := createMessage{Accounts: newAccounts}
			client.PrintInfo(message.String())
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
