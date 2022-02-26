// Copyright (c) 2020 IoTeX Foundation
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

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

var (
	abiOut string
	binOut string
)

// Multi-language support
var (
	contractCompileCmdUses = map[config.Language]string{
		config.English: "compile CONTRACT_NAME [CODE_FILES...] [--abi-out ABI_PATH] [--bin-out BIN_PATH]",
		config.Chinese: "compile 合约名 [代码文件...] [--abi-out ABI路径] [--bin-out BIN路径]",
	}
	contractCompileCmdShorts = map[config.Language]string{
		config.English: "Compile smart contract of IoTeX blockchain from source code file(s).",
		config.Chinese: "编译IoTeX区块链的智能合约代码,支持多文件编译",
	}
	flagAbiOutUsage = map[config.Language]string{
		config.English: "set abi file output path",
		config.Chinese: "设置abi文件输出路径",
	}
	flagBinOutUsage = map[config.Language]string{
		config.English: "set bin file output path",
		config.Chinese: "设置bin文件输出路径",
	}
)

// ContractCompileCmd represents the contract compile command
var ContractCompileCmd = &cobra.Command{
	Use:   config.TranslateInLang(contractCompileCmdUses, config.UILanguage),
	Short: config.TranslateInLang(contractCompileCmdShorts, config.UILanguage),
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := compile(args)
		return output.PrintError(err)
	},
}

func init() {
	ContractCompileCmd.Flags().StringVar(&abiOut, "abi-out", "",
		config.TranslateInLang(flagAbiOutUsage, config.UILanguage))

	ContractCompileCmd.Flags().StringVar(&binOut, "bin-out", "",
		config.TranslateInLang(flagBinOutUsage, config.UILanguage))
}

func compile(args []string) error {
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
		if name == contractName {
			break
		}
		nameSplit := strings.Split(name, ":")
		if nameSplit[len(nameSplit)-1] == contractName {
			contractName = name
			break
		}
	}

	contract, ok := contracts[contractName]
	if !ok {
		return output.NewError(output.CompilerError, fmt.Sprintf("failed to find out contract %s", contractName), nil)
	}

	abiByte, err := json.Marshal(contract.Info.AbiDefinition)
	if err != nil {
		return output.NewError(output.SerializationError, "failed to marshal abi", err)
	}

	result := []string{
		fmt.Sprintf("======= %s =======", contractName),
		fmt.Sprintf("Binary:\n%s", contract.Code),
		fmt.Sprintf("Contract JSON ABI\n%s", string(abiByte)),
	}
	output.PrintResult(strings.Join(result, "\n"))

	if binOut != "" {
		// bin file starts with "0x" prefix
		if err := os.WriteFile(binOut, []byte(contract.Code), 0600); err != nil {
			return output.NewError(output.WriteFileError, "failed to write bin file", err)
		}
	}

	if abiOut != "" {
		if err := os.WriteFile(abiOut, abiByte, 0600); err != nil {
			return output.NewError(output.WriteFileError, "failed to write abi file", err)
		}
	}

	return nil
}
