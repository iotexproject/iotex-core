// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	deploySolCmdUses = map[config.Language]string{
		config.English: "sol CODE_PATH CONTRACT_NAME [--with-arguments INIT_INPUT]",
		config.Chinese: "sol 源代码文件路径 合约名 [--with-arguments 初始化输入]",
	}
	deploySolCmdShorts = map[config.Language]string{
		config.English: "deploy smart contract with sol on IoTeX blockchain",
		config.Chinese: "deploy 使用 sol 方式在 IoTex区块链上部署智能合约",
	}
)

// contractDeploySolCmd represents the contract deploy sol command
var contractDeploySolCmd = &cobra.Command{
	Use:   config.TranslateInLang(deploySolCmdUses, config.UILanguage),
	Short: config.TranslateInLang(deploySolCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := contractDeploySol(args)
		return output.PrintError(err)
	},
}

func init() {
	withArgumentsFlag.RegisterCommand(contractDeploySolCmd)
}

func contractDeploySol(args []string) error {
	codePath := args[0]
	contractName := args[1]
	contracts, err := Compile(codePath)
	if err != nil {
		return output.NewError(0, "failed to compile", err)
	}

	contract, ok := contracts[contractName]
	if !ok {
		return output.NewError(output.CompilerError, fmt.Sprintf("failed to get contract from %s", contractName), nil)
	}

	bytecode, err := decodeBytecode(contract.Code)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to decode bytecode", err)
	}

	if withArgumentsFlag.Value().(string) != "" {
		abiByte, err := json.Marshal(contract.Info.AbiDefinition)
		if err != nil {
			return output.NewError(output.SerializationError, "failed to marshal abi", err)
		}

		abi, err := parseAbi(abiByte)
		if err != nil {
			return err
		}

		// Constructor's method name is "" (empty string)
		packedArg, err := packArguments(abi, "", withArgumentsFlag.Value().(string))
		if err != nil {
			return output.NewError(output.ConvertError, "failed to pack given arguments", err)
		}

		bytecode = append(bytecode, packedArg...)
	}

	amount := big.NewInt(0)
	return action.Execute("", amount, bytecode)
}
