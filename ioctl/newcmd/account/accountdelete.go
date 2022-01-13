// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/iotexproject/iotex-address/address"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	deleteShorts = map[config.Language]string{
		config.English: "Delete an IoTeX account/address from wallet/config",
		config.Chinese: "从 钱包/配置 中删除一个IoTeX的账户或地址",
	}
	deleteUses = map[config.Language]string{
		config.English: "delete [ALIAS|ADDRESS]",
		config.Chinese: "delete [别名|地址]",
	}
	failToGetAddress = map[config.Language]string{
		config.English: "failed to get address",
		config.Chinese: "获取账户地址失败",
	}
	failToConvertStringIntoAddress = map[config.Language]string{
		config.English: "failed to convert string into address",
		config.Chinese: "转换字符串到账户地址失败",
	}
	infoWarn = map[config.Language]string{
		config.English: "** This is an irreversible action!\n" +
			"Once an account is deleted, all the assets under this account may be lost!\n" +
			"Type 'YES' to continue, quit for anything else.",
		config.Chinese: "** 这是一个不可逆转的操作!\n" +
			"一旦一个账户被删除, 该账户下的所有资源都可能会丢失!\n" +
			"输入 'YES' 以继续, 否则退出",
	}
	failToRemoveKeystoreFile = map[config.Language]string{
		config.English: "failed to remove keystore file",
		config.Chinese: "移除keystore文件失败",
	}
	failToWriteToConfigFile = map[config.Language]string{
		config.English: "Failed to write to config file %s.",
		config.Chinese: "写入配置文件 %s 失败",
	}
	resultSuccess = map[config.Language]string{
		config.English: "Account #%s has been deleted.",
		config.Chinese: "账户 #%s 已被删除",
	}
	failToFindAccount = map[config.Language]string{
		config.English: "account #%s not found",
		config.Chinese: "账户 #%s 未找到",
	}
)

// NewAccountDelete represents the account delete command
func NewAccountDelete(c ioctl.Client) *cobra.Command {
	use, _ := c.SelectTranslation(deleteUses)
	short, _ := c.SelectTranslation(deleteShorts)
	failToGetAddress, _ := c.SelectTranslation(failToGetAddress)
	failToConvertStringIntoAddress, _ := c.SelectTranslation(failToConvertStringIntoAddress)
	infoWarn, _ := c.SelectTranslation(infoWarn)
	failToRemoveKeystoreFile, _ := c.SelectTranslation(failToRemoveKeystoreFile)
	failToWriteToConfigFile, _ := c.SelectTranslation(failToWriteToConfigFile)
	resultSuccess, _ := c.SelectTranslation(resultSuccess)
	failToFindAccount, _ := c.SelectTranslation(failToFindAccount)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			arg := ""
			if len(args) == 1 {
				arg = args[0]
			}
			addr, err := c.GetAddress(arg)
			if err != nil {
				return output.NewError(output.AddressError, failToGetAddress, err)
			}
			account, err := address.FromString(addr)
			if err != nil {
				return output.NewError(output.ConvertError, failToConvertStringIntoAddress, nil)
			}

			var filePath string
			if CryptoSm2 {
				if filePath == "" {
					filePath = filepath.Join(c.Config().Wallet, "sm2sk-"+account.String()+".pem")
				}
			} else {
				ks := c.NewKeyStore(c.Config().Wallet, keystore.StandardScryptN, keystore.StandardScryptP)
				for _, v := range ks.Accounts() {
					if bytes.Equal(account.Bytes(), v.Address.Bytes()) {
						filePath = v.URL.Path
						break
					}
				}
			}

			// check whether crypto file exists
			if _, err = os.Stat(filePath); err != nil {
				return output.NewError(output.ReadFileError, fmt.Sprintf(failToFindAccount, addr), nil)
			}
			var confirm string
			message := output.ConfirmationMessage{Info: infoWarn, Options: []string{"yes"}}
			fmt.Println(message.String())
			fmt.Scanf("%s", &confirm)
			if !strings.EqualFold(confirm, "yes") {
				output.PrintResult("quit")
				return nil
			}

			if err := os.Remove(filePath); err != nil {
				return output.NewError(output.ReadFileError, failToRemoveKeystoreFile, err)
			}

			aliases := c.GetAliasMap()
			cfg := c.Config()
			delete(cfg.Aliases, aliases[addr])
			out, err := yaml.Marshal(&cfg)
			if err != nil {
				return output.NewError(output.SerializationError, "", err)
			}
			if err := os.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
				return output.NewError(output.WriteFileError,
					fmt.Sprintf(failToWriteToConfigFile, config.DefaultConfigFile), err)
			}
			output.PrintResult(fmt.Sprintf(resultSuccess, addr))
			return nil
		},
	}
	return cmd
}
