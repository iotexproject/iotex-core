// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"github.com/ethereum/go-ethereum/common/compiler"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/flag"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

const _solCompiler = "solc"

// Flags
var (
	_initialAmountFlag = flag.NewStringVar("init-amount", "0", config.TranslateInLang(_flagInitialAmountUsage, config.UILanguage))
)

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
	if solc.Major == 0 && solc.Minor == 5 {
		return true
	}
	if solc.Major == 0 && solc.Minor == 4 && solc.Patch >= 24 {
		return true
	}
	return false
}
