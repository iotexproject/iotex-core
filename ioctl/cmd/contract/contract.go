// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"

	"github.com/iotexproject/iotex-core/ioctl/util"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/compiler"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/flag"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

const solCompiler = "solc"

// ErrInvalidArg indicates argument is invalid
var ErrInvalidArg = errors.New("invalid argument")

// Flags
var (
	sourceFlag = flag.NewStringVar("source", "",
		config.TranslateInLang(flagSourceUsage, config.UILanguage))
	withArgumentsFlag = flag.NewStringVar("with-arguments", "",
		config.TranslateInLang(flagWithArgumentsUsage, config.UILanguage))
)

// Multi-language support
var (
	contractCmdUses = map[config.Language]string{
		config.English: "contract",
		config.Chinese: "contract",
	}
	contractCmdShorts = map[config.Language]string{
		config.English: "Deal with smart contract of IoTeX blockchain",
		config.Chinese: "处理IoTeX区块链的智能合约",
	}
	flagEndpointUsages = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点",
	}
	flagInsecureUsages = map[config.Language]string{
		config.English: "insecure connection for once",
		config.Chinese: "一次不安全的连接",
	}
	flagSourceUsage = map[config.Language]string{
		config.English: "set source code file path",
		config.Chinese: "设定代码文件路径",
	}
	flagWithArgumentsUsage = map[config.Language]string{
		config.English: "pass arguments in JSON format",
		config.Chinese: "按照JSON格式传入参数",
	}
)

// ContractCmd represents the contract command
var ContractCmd = &cobra.Command{
	Use:   config.TranslateInLang(contractCmdUses, config.UILanguage),
	Short: config.TranslateInLang(contractCmdShorts, config.UILanguage),
}

func init() {
	ContractCmd.AddCommand(ContractPrepareCmd)
	ContractCmd.AddCommand(ContractCompileCmd)
	ContractCmd.AddCommand(contractDeployCmd)
	ContractCmd.AddCommand(contractInvokeCmd)
	ContractCmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, config.TranslateInLang(flagEndpointUsages, config.UILanguage))
	ContractCmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure,
		config.TranslateInLang(flagInsecureUsages, config.UILanguage))
}

// Compile compiles smart contract from source code
func Compile(sourceFiles ...string) (map[string]*compiler.Contract, error) {
	solc, err := compiler.SolidityVersion(solCompiler)
	if err != nil {
		return nil, output.NewError(output.CompilerError, "solidity compiler not ready", err)
	}
	if !checkCompilerVersion(solc) {
		return nil, output.NewError(output.CompilerError,
			fmt.Sprintf("unsupported solc version %d.%d.%d", solc.Major, solc.Minor, solc.Patch), nil)
	}

	contracts, err := compiler.CompileSolidity(solCompiler, sourceFiles...)
	if err != nil {
		return nil, output.NewError(output.CompilerError, "failed to compile", err)
	}
	return contracts, nil
}

func checkCompilerVersion(solc *compiler.Solidity) bool {
	// TODO: may support 0.5.0 later, need to make sure range of valid version
	if solc.Major == 0 && solc.Minor == 4 && solc.Patch >= 24 {
		return true
	}
	return false
}

func readAbiFile(abiFile string) (*abi.ABI, error) {
	abiBytes, err := ioutil.ReadFile(abiFile)
	if err != nil {
		return nil, output.NewError(output.ReadFileError, "failed to read abi file", err)
	}

	return parseAbi(abiBytes)
}

func parseAbi(abiBytes []byte) (*abi.ABI, error) {
	parsedAbi, err := abi.JSON(strings.NewReader(string(abiBytes)))
	if err != nil {
		return nil, output.NewError(output.SerializationError, "failed to unmarshal abi", err)
	}
	return &parsedAbi, nil
}

func packArguments(targetAbi *abi.ABI, targetMethod string, rowInput string) ([]byte, error) {
	rowArguments, err := parseInput(rowInput)
	if err != nil {
		return nil, err
	}

	method, ok := targetAbi.Methods[targetMethod]
	if !ok {
		return nil, output.NewError(output.InputError, "invalid method name", nil)
	}

	arguments := make([]interface{}, 0, len(method.Inputs))
	for _, param := range method.Inputs {
		rowArg, ok := rowArguments[param.Name]
		if !ok {
			return nil, output.NewError(output.InputError, fmt.Sprintf("failed to parse argument \"%s\"", param.Name), nil)
		}
		arg, err := parseArgument(&param.Type, rowArg)
		if err != nil {
			return nil, output.NewError(output.InputError, fmt.Sprintf("failed to parse argument \"%s\"", param.Name), err)
		}
		arguments = append(arguments, arg)
	}
	fmt.Println(reflect.ValueOf(arguments[0]).Type(), reflect.ValueOf(arguments[0]).Kind())
	return targetAbi.Pack(targetMethod, arguments...)
}

func parseInput(rowInput string) (map[string]interface{}, error) {
	var input map[string]interface{}
	if err := json.Unmarshal([]byte(rowInput), &input); err != nil {
		return nil, output.NewError(output.SerializationError, "failed to unmarshal arguments", err)
	}
	return input, nil
}

func parseArgument(t *abi.Type, arg interface{}) (interface{}, error) {
	fmt.Println(t.Type, t.Kind, t.Elem, t.Size, t.T)
	// TODO: more type handler needed
	switch t.T {
	case abi.SliceTy, abi.ArrayTy:
		// TODO: this slice solution cannot be accepted by ether's abi.Pack
		if reflect.TypeOf(arg).Kind() != reflect.Slice {
			return nil, ErrInvalidArg
		}

		list := make([]interface{}, 0, t.Size)
		s := reflect.ValueOf(arg)
		for i := 0; i < s.Len(); i++ {
			ele, err := parseArgument(t.Elem, s.Index(i).Interface())
			if err != nil {
				return nil, ErrInvalidArg
			}
			list = append(list, ele)
		}
		arg = list
	case abi.AddressTy:
		addrString, ok := arg.(string)
		if !ok {
			return nil, ErrInvalidArg
		}

		addr, err := util.IoAddrToEvmAddr(addrString)
		if err != nil {
			return nil, err
		}

		arg = addr
	}
	return arg, nil
}
