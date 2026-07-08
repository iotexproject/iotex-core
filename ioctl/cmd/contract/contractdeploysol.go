// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/flag"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

// Multi-language support
var (
	_deploySolCmdUses = map[config.Language]string{
		config.English: "sol [FILE_NAME:]CONTRACT_NAME [CODE_FILES_OR_DIRS...] [--with-arguments INIT_INPUT] [--amount IOTX]",
		config.Chinese: "sol [文件名:]合约名 [代码文件或目录...] [--with-arguments 初始化输入] [--amount IOTX数量]",
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
	aliasInitAmount(_contractDeploySolCmd)
}

func contractDeploySol(args []string) error {
	contractName := args[0]

	// Sources may be an explicit list of .sol files and/or directories. When no
	// source is given, default to scanning the current directory (legacy behavior).
	sources := args[1:]
	if len(sources) == 0 {
		sources = []string{"."}
	}
	files, err := expandSolSources(sources)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return output.NewError(output.InputError, "failed to get source file(s)", nil)
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

// expandSolSources expands any directory entries in sources into the .sol files
// they contain (non-recursively), leaving explicit file paths untouched. This
// lets `contract sol` accept a directory in addition to an explicit file list,
// while remaining backward compatible with the previous file-list-only usage.
func expandSolSources(sources []string) ([]string, error) {
	var files []string
	for _, src := range sources {
		info, err := os.Stat(src)
		if err != nil {
			return nil, output.NewError(output.ReadFileError,
				fmt.Sprintf("failed to access source %q", src), err)
		}
		if !info.IsDir() {
			files = append(files, src)
			continue
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return nil, output.NewError(output.ReadFileError,
				fmt.Sprintf("failed to read directory %q", src), err)
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".sol") {
				files = append(files, filepath.Join(src, e.Name()))
			}
		}
	}
	return files, nil
}
