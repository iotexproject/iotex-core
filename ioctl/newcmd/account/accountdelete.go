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
	_deleteShorts = map[config.Language]string{
		config.English: "Delete an IoTeX account/address from wallet/config",
		config.Chinese: "从 钱包/配置 中删除一个IoTeX的账户或地址",
	}
	_deleteUses = map[config.Language]string{
		config.English: "delete [ALIAS|ADDRESS]",
		config.Chinese: "delete [别名|地址]",
	}
	_failToGetAddress = map[config.Language]string{
		config.English: "failed to get address",
		config.Chinese: "获取账户地址失败",
	}
	_failToConvertStringIntoAddress = map[config.Language]string{
		config.English: "failed to convert string into address",
		config.Chinese: "转换字符串到账户地址失败",
	}
	_infoWarn = map[config.Language]string{
		config.English: "** This is an irreversible action!\n" +
			"Once an account is deleted, all the assets under this account may be lost!\n" +
			"Type 'YES' to continue, quit for anything else.",
		config.Chinese: "** 这是一个不可逆转的操作!\n" +
			"一旦一个账户被删除, 该账户下的所有资源都可能会丢失!\n" +
			"输入 'YES' 以继续, 否则退出",
	}
	_infoQuit = map[config.Language]string{
		config.English: "quit",
		config.Chinese: "退出",
	}
	_failToRemoveKeystoreFile = map[config.Language]string{
		config.English: "failed to remove keystore file",
		config.Chinese: "移除keystore文件失败",
	}
	_failToWriteToConfigFile = map[config.Language]string{
		config.English: "Failed to write to config file.",
		config.Chinese: "写入配置文件失败",
	}
	_resultSuccess = map[config.Language]string{
		config.English: "Account #%s has been deleted.",
		config.Chinese: "账户 #%s 已被删除",
	}
	_failToFindAccount = map[config.Language]string{
		config.English: "account #%s not found",
		config.Chinese: "账户 #%s 未找到",
	}
)

// NewAccountDelete represents the account delete command
func NewAccountDelete(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_deleteUses)
	short, _ := client.SelectTranslation(_deleteShorts)
	_failToGetAddress, _ := client.SelectTranslation(_failToGetAddress)
	_failToConvertStringIntoAddress, _ := client.SelectTranslation(_failToConvertStringIntoAddress)
	_infoWarn, _ := client.SelectTranslation(_infoWarn)
	_failToRemoveKeystoreFile, _ := client.SelectTranslation(_failToRemoveKeystoreFile)
	_failToWriteToConfigFile, _ := client.SelectTranslation(_failToWriteToConfigFile)
	_resultSuccess, _ := client.SelectTranslation(_resultSuccess)
	_failToFindAccount, _ := client.SelectTranslation(_failToFindAccount)
	_infoQuit, _ := client.SelectTranslation(_infoQuit)

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

			addr, err := client.AddressWithDefaultIfNotExist(arg)
			if err != nil {
				return errors.Wrap(err, _failToGetAddress)
			}
			account, err := address.FromString(addr)
			if err != nil {
				return errors.Wrap(err, _failToConvertStringIntoAddress)
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
				return errors.Wrapf(err, _failToFindAccount, addr)
			}
			confirmed, err := client.AskToConfirm(_infoWarn)
			if err != nil {
				return errors.Wrap(err, "failed to ask confirm")
			}
			if !confirmed {
				cmd.Println(_infoQuit)
				return nil
			}
			if err = os.Remove(filePath); err != nil {
				return errors.Wrap(err, _failToRemoveKeystoreFile)
			}
			if err = client.DeleteAlias(client.AliasMap()[addr]); err != nil {
				return errors.Wrap(err, _failToWriteToConfigFile)
			}
			cmd.Println(fmt.Sprintf(_resultSuccess, addr))
			return nil
		},
	}
}
