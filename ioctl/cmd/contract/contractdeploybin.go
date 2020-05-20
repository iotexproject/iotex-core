// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"io/ioutil"
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	deployBinCmdUses = map[config.Language]string{
		config.English: "bin BIN_PATH [ABI_PATH INIT_INPUT]",
		config.Chinese: "bin BIN文件路径 [ABI文件路径 初始化输入]",
	}
	deployBinCmdShorts = map[config.Language]string{
		config.English: "deploy smart contract with bin on IoTeX blockchain",
		config.Chinese: "deploy 使用 bin 文件方式在 IoTex区块链上部署智能合约",
	}
)

// contractDeployBinCmd represents the contract deploy bin command
var contractDeployBinCmd = &cobra.Command{
	Use:   config.TranslateInLang(deployBinCmdUses, config.UILanguage),
	Short: config.TranslateInLang(deployBinCmdShorts, config.UILanguage),
	Args:  util.CheckArgs(1, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := contractDeployBin(args)
		return output.PrintError(err)
	},
}

func contractDeployBin(args []string) error {
	bin, err := ioutil.ReadFile(args[0])
	if err != nil {
		return output.NewError(output.ReadFileError, "failed to read bin file", err)
	}
	bytecode, err := decodeBytecode(string(bin))
	if err != nil {
		return output.NewError(output.ConvertError, "failed to decode bytecode", err)
	}
	if len(args) == 3 {
		abi, err := readAbiFile(args[1])
		if err != nil {
			return err
		}

		// Constructor's method name is "" (empty string)
		packedArg, err := packArguments(abi, "", args[2])
		if err != nil {
			return output.NewError(output.ConvertError, "failed to pack given arguments", err)
		}

		bytecode = append(bytecode, packedArg...)
	}
	amount := big.NewInt(0)
	return action.Execute("", amount, bytecode)
}
