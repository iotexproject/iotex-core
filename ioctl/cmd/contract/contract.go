// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/compiler"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/flag"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

const _solCompiler = "solc"

// Flags
var (
	_initialAmountFlag = flag.NewStringVar("init-amount", "0",
		config.TranslateInLang(_flagInitialAmountUsage, config.UILanguage))
)

// Multi-language support
var (
	_contractCmdShorts = map[config.Language]string{
		config.English: "Deal with smart contract of IoTeX blockchain",
		config.Chinese: "处理IoTeX区块链的智能合约",
	}
	_flagEndpointUsages = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点",
	}
	_flagInsecureUsages = map[config.Language]string{
		config.English: "insecure connection for once",
		config.Chinese: "一次不安全的连接",
	}
	_flagInitialAmountUsage = map[config.Language]string{
		config.English: "transfer an initial amount to the new deployed contract",
		config.Chinese: "为部署的新合约转入一笔初始资金",
	}
)

// ContractCmd represents the contract command
var ContractCmd = &cobra.Command{
	Use:   "contract",
	Short: config.TranslateInLang(_contractCmdShorts, config.UILanguage),
}

func init() {
	ContractCmd.AddCommand(ContractPrepareCmd)
	ContractCmd.AddCommand(ContractCompileCmd)
	ContractCmd.AddCommand(_contractDeployCmd)
	ContractCmd.AddCommand(_contractInvokeCmd)
	ContractCmd.AddCommand(_contractTestCmd)
	ContractCmd.AddCommand(_contractShareCmd)
	ContractCmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, config.TranslateInLang(_flagEndpointUsages, config.UILanguage))
	ContractCmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure,
		config.TranslateInLang(_flagInsecureUsages, config.UILanguage))

	flag.WithArgumentsFlag.RegisterCommand(_contractDeploySolCmd)
	flag.WithArgumentsFlag.RegisterCommand(_contractInvokeFunctionCmd)
	flag.WithArgumentsFlag.RegisterCommand(_contractTestFunctionCmd)
}

// Compile compiles smart contract from source code
func Compile(sourceFiles ...string) (map[string]*compiler.Contract, error) {
	solc, err := util.SolidityVersion(_solCompiler)
	if err != nil {
		return nil, output.NewError(output.CompilerError, "solidity compiler not ready", err)
	}
	if !checkCompilerVersion(solc) {
		return nil, output.NewError(output.CompilerError,
			fmt.Sprintf("unsupported solc version %d.%d.%d", solc.Major, solc.Minor, solc.Patch), nil)
	}

	contracts, err := util.CompileSolidity(_solCompiler, sourceFiles...)
	if err != nil {
		return nil, output.NewError(output.CompilerError, "failed to compile", err)
	}
	return contracts, nil
}

func checkCompilerVersion(solc *util.Solidity) bool {
	if solc.Major == 0 && solc.Minor == 5 {
		return true
	}
	if solc.Major == 0 && solc.Minor == 4 && solc.Patch >= 24 {
		return true
	}
	return false
}

func readAbiFile(abiFile string) (*abi.ABI, error) {
	abiBytes, err := os.ReadFile(filepath.Clean(abiFile))
	if err != nil {
		return nil, output.NewError(output.ReadFileError, "failed to read abi file", err)
	}

	return parseAbi(abiBytes)
}

func packArguments(targetAbi *abi.ABI, targetMethod string, rowInput string) ([]byte, error) {
	var method abi.Method
	var ok bool

	if rowInput == "" {
		rowInput = "{}"
	}

	rowArguments, err := parseInput(rowInput)
	if err != nil {
		return nil, err
	}

	if targetMethod == "" {
		method = targetAbi.Constructor
	} else {
		method, ok = targetAbi.Methods[targetMethod]
		if !ok {
			return nil, output.NewError(output.InputError, "invalid method name", nil)
		}
	}

	arguments := make([]interface{}, 0, len(method.Inputs))
	for _, param := range method.Inputs {
		if param.Name == "" {
			param.Name = "_"
		}

		rowArg, ok := rowArguments[param.Name]
		if !ok {
			return nil, output.NewError(output.InputError, fmt.Sprintf("failed to parse argument \"%s\"", param.Name), nil)
		}

		arg, err := parseInputArgument(&param.Type, rowArg)
		if err != nil {
			return nil, output.NewError(output.InputError, fmt.Sprintf("failed to parse argument \"%s\"", param.Name), err)
		}
		arguments = append(arguments, arg)
	}
	return targetAbi.Pack(targetMethod, arguments...)
}

func decodeBytecode(bytecode string) ([]byte, error) {
	return hex.DecodeString(util.TrimHexPrefix(bytecode))
}
