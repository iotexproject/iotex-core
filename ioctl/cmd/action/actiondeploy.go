// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	deployCmdShorts = map[config.Language]string{
		config.English: "Deploy smart contract on IoTeX blockchain",
		config.Chinese: "在IoTeX区块链上部署智能合约",
	}
	deployCmdUses = map[config.Language]string{
		config.English: "deploy [AMOUNT_IOTX] [-s SIGNER] -b BYTE_CODE [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "deploy [IOTX数量] [-s 签署人] -b 类型码 [-n NONCE] [-l GAS限制] [-p GAS价格] [-P" +
			" 密码] [-y]",
	}
)

// actionDeployCmd represents the action deploy command
var actionDeployCmd = &cobra.Command{
	Use:   config.TranslateInLang(deployCmdUses, config.UILanguage),
	Short: config.TranslateInLang(deployCmdShorts, config.UILanguage),
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := deploy(args)
		return output.PrintError(err)
	},
}

func init() {
	registerWriteCommand(actionDeployCmd)
	bytecodeFlag.RegisterCommand(actionDeployCmd)
	bytecodeFlag.MarkFlagRequired(actionDeployCmd)
}

func deploy(args []string) error {
	bytecode, err := decodeBytecode()
	if err != nil {
		return output.NewError(output.FlagError, "invalid bytecode flag", err)
	}
	amount := big.NewInt(0)
	if len(args) == 1 {
		amount, err = util.StringToRau(args[0], util.IotxDecimalNum)
		if err != nil {
			return output.NewError(output.ConvertError, "invalid amount", err)
		}
	}
	return Execute("", amount, bytecode)
}
