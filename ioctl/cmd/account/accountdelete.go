// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package account

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	_deleteCmdShorts = map[config.Language]string{
		config.English: "Delete an IoTeX account/address from wallet/config",
		config.Chinese: "从 钱包/配置 中删除一个IoTeX的账户或地址",
	}
	_deleteCmdUses = map[config.Language]string{
		config.English: "delete [ALIAS|ADDRESS]",
		config.Chinese: "delete [别名|地址]",
	}
)

// _accountDeleteCmd represents the account delete command
var _accountDeleteCmd = &cobra.Command{
	Use:   config.TranslateInLang(_deleteCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_deleteCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		arg := ""
		if len(args) == 1 {
			arg = args[0]
		}
		err := accountDelete(arg)
		return output.PrintError(err)
	},
}

func accountDelete(arg string) error {
	addr, err := util.GetAddress(arg)
	if err != nil {
		return output.NewError(output.AddressError, "failed to get address", err)
	}
	account, err := address.FromString(addr)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to convert string into address", nil)
	}

	var filePath string
	if CryptoSm2 {
		if filePath == "" {
			filePath = filepath.Join(config.ReadConfig.Wallet, "sm2sk-"+account.String()+".pem")
		}
	} else {
		ks := keystore.NewKeyStore(config.ReadConfig.Wallet,
			keystore.StandardScryptN, keystore.StandardScryptP)
		for _, v := range ks.Accounts() {
			if bytes.Equal(account.Bytes(), v.Address.Bytes()) {
				filePath = v.URL.Path
				break
			}
		}
	}

	// check whether crypto file exists
	_, err = os.Stat(filePath)
	if err != nil {
		return output.NewError(output.CryptoError, fmt.Sprintf("crypto file of account #%s not found", addr), err)
	}

	var confirm string
	info := fmt.Sprintf("** This is an irreversible action!\n" +
		"Once an account is deleted, all the assets under this account may be lost!\n" +
		"Type 'YES' to continue, quit for anything else.")
	message := output.ConfirmationMessage{Info: info, Options: []string{"yes"}}
	fmt.Println(message.String())
	if _, err := fmt.Scanf("%s", &confirm); err != nil {
		return output.NewError(output.InputError, "failed to input yes", err)
	}
	if !strings.EqualFold(confirm, "yes") {
		output.PrintResult("quit")
		return nil
	}

	if err := os.Remove(filePath); err != nil {
		return output.NewError(output.WriteFileError, "failed to remove keystore or pem file", err)
	}

	aliases := alias.GetAliasMap()
	delete(config.ReadConfig.Aliases, aliases[addr])
	out, err := yaml.Marshal(&config.ReadConfig)
	if err != nil {
		return output.NewError(output.SerializationError, "", err)
	}
	if err := os.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		return output.NewError(output.WriteFileError,
			fmt.Sprintf("Failed to write to config file %s.", config.DefaultConfigFile), err)
	}

	output.PrintResult(fmt.Sprintf("Account #%s has been deleted.", addr))
	return nil
}
