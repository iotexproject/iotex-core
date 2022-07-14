// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	_hdwalletDeleteCmdShorts = map[config.Language]string{
		config.English: "delete hdwallet",
		config.Chinese: "删除钱包",
	}
)

// _hdwalletDeleteCmd represents the hdwallet delete command
var _hdwalletDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: config.TranslateInLang(_hdwalletDeleteCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := hdwalletDelete()
		return output.PrintError(err)
	},
}

func hdwalletDelete() error {
	var confirm string
	info := fmt.Sprintf("** This is an irreversible action!\n" +
		"Once an hdwallet is deleted, all the assets under this hdwallet may be lost!\n" +
		"Type 'YES' to continue, quit for anything else.")
	message := output.ConfirmationMessage{Info: info, Options: []string{"yes"}}
	fmt.Println(message.String())
	fmt.Scanf("%s", &confirm)
	if !strings.EqualFold(confirm, "yes") {
		output.PrintResult("quit")
		return nil
	}

	return os.Remove(_hdWalletConfigFile)
}
