// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"fmt"
	"os/exec"

	"github.com/ethereum/go-ethereum/common/compiler"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	_prepareCmdShorts = map[config.Language]string{
		config.English: "Prepare solidity compiler",
		config.Chinese: "准备solidity编译器",
	}
)

// ContractPrepareCmd represents the contract prepare command
var ContractPrepareCmd = &cobra.Command{
	Use:   "prepare",
	Short: config.TranslateInLang(_prepareCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := prepare()
		return err
	},
}

func prepare() error {
	_, err := compiler.SolidityVersion(_solCompiler)
	if err != nil {
		cmdString := "curl --silent https://raw.githubusercontent.com/iotexproject/iotex-core/master/install-solc.sh | sh"
		cmd := exec.Command("bash", "-c", cmdString)
		output.PrintResult("Preparing solidity compiler ...\n")

		err = cmd.Run()
		if err != nil {
			return output.NewError(output.UpdateError, "failed to prepare solc", err)
		}
	}
	solc, _ := compiler.SolidityVersion(_solCompiler)

	if !checkCompilerVersion(solc) {
		return output.NewError(output.CompilerError,
			fmt.Sprintf("unsupported solc version %d.%d.%d, expects solc version 0.5.17",
				solc.Major, solc.Minor, solc.Patch), nil)
	}

	output.PrintResult("Solidity compiler is ready now.")
	return nil
}
