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

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
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
	infoQuit = map[config.Language]string{
		config.English: "quit",
		config.Chinese: "退出",
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
func NewAccountDelete(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(deleteUses)
	short, _ := client.SelectTranslation(deleteShorts)
	failToGetAddress, _ := client.SelectTranslation(failToGetAddress)
	failToConvertStringIntoAddress, _ := client.SelectTranslation(failToConvertStringIntoAddress)
	infoWarn, _ := client.SelectTranslation(infoWarn)
	failToRemoveKeystoreFile, _ := client.SelectTranslation(failToRemoveKeystoreFile)
	failToWriteToConfigFile, _ := client.SelectTranslation(failToWriteToConfigFile)
	resultSuccess, _ := client.SelectTranslation(resultSuccess)
	failToFindAccount, _ := client.SelectTranslation(failToFindAccount)
	infoQuit, _ := client.SelectTranslation(infoQuit)

	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			arg := ""
			if len(args) == 1 {
				arg = args[0]
			}

			addr, err := client.GetAddress(arg)
			if err != nil {
				return errors.Wrap(err, failToGetAddress)
			}
			account, err := address.FromString(addr)
			if err != nil {
				return errors.Wrap(err, failToConvertStringIntoAddress)
			}

			var filePath string
			if client.IsCryptoSm2() {
				if filePath == "" {
					filePath = filepath.Join(client.Config().Wallet, "sm2sk-"+account.String()+".pem")
				}
			} else {
				ks := client.NewKeyStore()
				for _, v := range ks.Accounts() {
					if bytes.Equal(account.Bytes(), v.Address.Bytes()) {
						filePath = v.URL.Path
						break
					}
				}
			}

			// check whether crypto file exists
			if _, err = os.Stat(filePath); err != nil {
				return errors.Wrapf(err, failToFindAccount, addr)
			}
			if !client.AskToConfirm(infoWarn) {
				client.PrintInfo(infoQuit)
				return nil
			}
			if err := os.Remove(filePath); err != nil {
				return errors.Wrap(err, failToRemoveKeystoreFile)
			}
			if err := client.DeleteAlias(client.AliasMap()[addr]); err != nil {
				return errors.Wrapf(err, failToWriteToConfigFile, config.DefaultConfigFile)
			}
			client.PrintInfo(fmt.Sprintf(resultSuccess, addr))
			return nil
		},
	}
}
