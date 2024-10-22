// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contract

import (
	"os/exec"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

// Multi-language support
var (
	_prepareCmdShorts = map[config.Language]string{
		config.English: "Prepare solidity compiler",
		config.Chinese: "准备solidity编译器",
	}
)

// NewContractPrepareCmd represents the contract prepare command
func NewContractPrepareCmd(client ioctl.Client) *cobra.Command {
	short, _ := client.SelectTranslation(_prepareCmdShorts)

	return &cobra.Command{
		Use:   "prepare",
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			_, err := util.SolidityVersion(_solCompiler)
			if err != nil {
				cmdString := "curl --silent https://raw.githubusercontent.com/iotexproject/iotex-core/master/install-solc.sh | sh"
				installCmd := exec.Command("bash", "-c", cmdString)
				cmd.Println("Preparing solidity compiler ...")

				err = installCmd.Run()
				if err != nil {
					return errors.Wrap(err, "failed to prepare solc")
				}
			}
			solc, err := util.SolidityVersion(_solCompiler)
			if err != nil {
				return errors.Wrap(err, "solidity compiler not ready")
			}
			if !checkCompilerVersion(solc) {
				return errors.Errorf("unsupported solc version %d.%d.%d, expects solc version 0.8.17\n",
					solc.Major, solc.Minor, solc.Patch)
			}

			cmd.Println("Solidity compiler is ready now.")
			return nil
		},
	}
}
