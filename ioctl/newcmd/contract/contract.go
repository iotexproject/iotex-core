// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contract

import (
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/compiler"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

const _solCompiler = "solc"

// Multi-language support
var (
	_contractCmdShorts = map[config.Language]string{
		config.English: "Deal with smart contract of IoTeX blockchain",
		config.Chinese: "处理IoTeX区块链的智能合约",
	}
	_flagInitialAmountUsage = map[config.Language]string{
		config.English: "transfer an initial amount to the new deployed contract",
		config.Chinese: "为部署的新合约转入一笔初始资金",
	}
)

// NewContractCmd represents the contract command
func NewContractCmd(client ioctl.Client) *cobra.Command {
	short, _ := client.SelectTranslation(_contractCmdShorts)
	cmd := &cobra.Command{
		Use:   "contract",
		Short: short,
	}

	// TODO add sub commands
	// cmd.AddCommand(NewContractPrepareCmd)
	// cmd.AddCommand(NewContractCompileCmd)
	// cmd.AddCommand(NewContractDeployCmd)
	// cmd.AddCommand(NewContractInvokeCmd)
	// cmd.AddCommand(NewContractTestCmd)
	// cmd.AddCommand(NewContractShareCmd)
	cmd.AddCommand(NewContractCompileCmd(client))
	client.SetEndpointWithFlag(cmd.PersistentFlags().StringVar)
	client.SetInsecureWithFlag(cmd.PersistentFlags().BoolVar)

	return cmd
}

// Compile compiles smart contract from source code
func Compile(sourceFiles ...string) (map[string]*compiler.Contract, error) {
	solc, err := util.SolidityVersion(_solCompiler)
	if err != nil {
		return nil, errors.Wrap(err, "solidity compiler not ready")
	}
	if !checkCompilerVersion(solc) {
		return nil, errors.Errorf("unsupported solc version %d.%d.%d", solc.Major, solc.Minor, solc.Patch)
	}

	contracts, err := util.CompileSolidity(_solCompiler, sourceFiles...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile")
	}
	return contracts, nil
}

func checkCompilerVersion(solc *util.Solidity) bool {
	if solc.Major == 0 && solc.Minor == 8 {
		return true
	}
	return false
}

func decodeBytecode(bytecode string) ([]byte, error) {
	return hex.DecodeString(util.TrimHexPrefix(bytecode))
}

func readAbiFile(abiFile string) (*abi.ABI, error) {
	abiBytes, err := os.ReadFile(filepath.Clean(abiFile))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read abi file")
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
			return nil, errors.New("invalid method name")
		}
	}

	arguments := make([]interface{}, 0, len(method.Inputs))
	for _, param := range method.Inputs {
		if param.Name == "" {
			param.Name = "_"
		}

		rowArg, ok := rowArguments[param.Name]
		if !ok {
			return nil, errors.Errorf("failed to parse argument \"%s\"", param.Name)
		}

		arg, err := parseInputArgument(&param.Type, rowArg)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse argument \"%s\"", param.Name)
		}
		arguments = append(arguments, arg)
	}
	return targetAbi.Pack(targetMethod, arguments...)
}
