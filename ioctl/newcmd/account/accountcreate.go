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
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	createShorts = map[config.Language]string{
		config.English: "Create N new accounts and print them",
		config.Chinese: "创建 N 个新账户，并打印",
	}
	createUses = map[config.Language]string{
		config.English: "create",
		config.Chinese: "create 创建",
	}
	createFlagUsages = map[config.Language]string{
		config.English: "number of accounts to create",
		config.Chinese: "指定创建账户的数量",
	}
	failToGenerateNewPrivateKey = map[config.Language]string{
		config.English: "failed to generate new private key",
		config.Chinese: "生成新私钥失败",
	}
	failToGenerateNewPrivateKeySm2 = map[config.Language]string{
		config.English: "failed to generate new sm2 private key",
		config.Chinese: "生成新sm2私钥失败",
	}
	failToConvertPublicKeyIntoAddress = map[config.Language]string{
		config.English: "failed to convert public key into address",
		config.Chinese: "将公钥转换为地址失败",
	}
)

// NewAccountCreate represents the account create command
func NewAccountCreate(c ioctl.Client) *cobra.Command {
	var numAccounts uint
	use, _ := c.SelectTranslation(createUses)
	short, _ := c.SelectTranslation(createShorts)
	usage, _ := c.SelectTranslation(createFlagUsages)
	failToGenerateNewPrivateKey, _ := c.SelectTranslation(failToGenerateNewPrivateKey)
	failToGenerateNewPrivateKeySm2, _ := c.SelectTranslation(failToGenerateNewPrivateKeySm2)
	failToConvertPublicKeyIntoAddress, _ := c.SelectTranslation(failToConvertPublicKeyIntoAddress)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
      cmd.SilenceErrors = true
			var err error
			var private crypto.PrivateKey
      
			newAccounts := make([]generatedAccount, 0)
			for i := 0; i < int(numAccounts); i++ {
				if !CryptoSm2 {
					private, err = crypto.GenerateKey()
					if err != nil {
						c.PrintError(output.NewError(output.CryptoError, failToGenerateNewPrivateKey, err))
					  return nil
					}
				} else {
					private, err = crypto.GenerateKeySm2()
					if err != nil {
            c.PrintError(output.NewError(output.CryptoError, failToGenerateNewPrivateKeySm2, err))
					  return nil
					}
				}

				addr := private.PublicKey().Address()
				if addr == nil {
					c.PrintError(output.NewError(output.AddressError, failToConvertPublicKeyIntoAddress, nil))
					return nil
				}
				newAccount := generatedAccount{
					Address:    addr.String(),
					PrivateKey: fmt.Sprintf("%x", private.Bytes()),
					PublicKey:  fmt.Sprintf("%x", private.PublicKey().Bytes()),
				}
				newAccounts = append(newAccounts, newAccount)
			}

			message := createMessage{Accounts: newAccounts}

			c.PrintInfo(message.String())
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
	if output.Format == "" {
		byteAsJSON, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			log.Panic(err)
		}
		return fmt.Sprint(string(byteAsJSON))
	}
	return output.FormatString(output.Result, m)
}
