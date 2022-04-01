// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/flag"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	_deploySolCmdUses = map[config.Language]string{
		config.English: "sol [FILE_NAME:]CONTRACT_NAME [CODE_FILES...] [--with-arguments INIT_INPUT] [--init-amount IOTX数量]",
		config.Chinese: "sol [文件名:]合约名 [代码文件...] [--with-arguments 初始化输入] [--init-amount IOTX数量]",
	}
	_deploySolCmdShorts = map[config.Language]string{
		config.English: "deploy smart contract with sol files on IoTeX blockchain",
		config.Chinese: "使用sol文件在IoTex区块链上部署智能合约",
	}
)

// _contractDeploySolCmd represents the contract deploy sol command
var _contractDeploySolCmd = &cobra.Command{
	Use:   config.TranslateInLang(_deploySolCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_deploySolCmdShorts, config.UILanguage),
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := contractDeploySol(args)
		return output.PrintError(err)
	},
}

func init() {
	_initialAmountFlag.RegisterCommand(_contractDeploySolCmd)
}

func contractDeploySol(args []string) error {
	contractName := args[0]

	files := args[1:]
	if len(files) == 0 {
		dirInfo, err := os.ReadDir("./")
		if err != nil {
			return output.NewError(output.ReadFileError, "failed to get current directory", err)
		}

		for _, fileInfo := range dirInfo {
			if !fileInfo.IsDir() && strings.HasSuffix(fileInfo.Name(), ".sol") {
				files = append(files, fileInfo.Name())
			}
		}

		if len(files) == 0 {
			return output.NewError(output.InputError, "failed to get source file(s)", nil)
		}
	}

	contracts, err := Compile(files...)
	if err != nil {
		return output.NewError(0, "failed to compile", err)
	}

	for name := range contracts {
		if strings.HasSuffix(name, contractName) {
			if contractName != args[0] {
				return output.NewError(output.CompilerError,
					fmt.Sprintf("there are more than one %s contract", args[0]), nil)
			}
			contractName = name
		}
	}

	contract, ok := contracts[contractName]
	if !ok {
		return output.NewError(output.CompilerError, fmt.Sprintf("failed to find out contract %s", contractName), nil)
	}

	bytecode, err := decodeBytecode(contract.Code)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to decode bytecode", err)
	}

	if flag.WithArgumentsFlag.Value().(string) != "" {
		abiByte, err := json.Marshal(contract.Info.AbiDefinition)
		if err != nil {
			return output.NewError(output.SerializationError, "failed to marshal abi", err)
		}

		abi, err := parseAbi(abiByte)
		if err != nil {
			return err
		}

		// Constructor's method name is "" (empty string)
		packedArg, err := packArguments(abi, "", flag.WithArgumentsFlag.Value().(string))
		if err != nil {
			return output.NewError(output.ConvertError, "failed to pack given arguments", err)
		}

		bytecode = append(bytecode, packedArg...)
	}

	amount, err := util.StringToRau(_initialAmountFlag.Value().(string), util.IotxDecimalNum)
	if err != nil {
		return output.NewError(output.FlagError, "invalid amount", err)
	}

	return action.Execute("", amount, bytecode)
}
