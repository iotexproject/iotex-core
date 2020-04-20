// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common/compiler"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/flag"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

const solCompiler = "solc"

// Flags
var (
	sourceFlag = flag.NewStringVar("source", "",
		config.TranslateInLang(flagSourceUsage, config.UILanguage))
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
	flagVersionUsage = map[config.Language]string{
		config.English: "set solidity version",
		config.Chinese: "设定solidity版本",
	}
)

// ContractCmd represents the contract command
var ContractCmd = &cobra.Command{
	Use:   config.TranslateInLang(contractCmdUses, config.UILanguage),
	Short: config.TranslateInLang(contractCmdShorts, config.UILanguage),
}

func init() {
	ContractCmd.AddCommand(ContractCompileCmd)
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
