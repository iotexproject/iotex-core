// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package did

import (
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	updateCmdUses = map[config.Language]string{
		config.English: "update (CONTRACT_ADDRESS|ALIAS) hash uri",
		config.Chinese: "update (合约地址|别名) hash uri",
	}
	updateCmdShorts = map[config.Language]string{
		config.English: "Update DID on IoTeX blockchain",
		config.Chinese: "Update 在IoTeX链上更新DID",
	}
)

// didUpdateCmd represents the contract invoke update command
var didUpdateCmd = &cobra.Command{
	Use:   config.TranslateInLang(updateCmdUses, config.UILanguage),
	Short: config.TranslateInLang(updateCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := updateDID(args)
		return output.PrintError(err)
	},
}

func init() {
	action.RegisterWriteCommand(didUpdateCmd)
}

func updateDID(args []string) error {
	contract, err := util.Address(args[0])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get contract address", err)
	}

	bytecode, err := encode(_updateDIDName, args[1], args[2])
	if err != nil {
		return output.NewError(output.ConvertError, "invalid bytecode", err)
	}

	return action.Execute(contract, big.NewInt(0), bytecode)
}
