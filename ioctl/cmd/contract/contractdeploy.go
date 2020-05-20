// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"encoding/hex"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	deployCmdUses = map[config.Language]string{
		config.English: "deploy",
		config.Chinese: "deploy",
	}
	deployCmdShorts = map[config.Language]string{
		config.English: "deploy smart contract of IoTeX blockchain",
		config.Chinese: "在IoTeX区块链部署智能合约",
	}
)

// contractDeployCmd represents the contract deploy command
var contractDeployCmd = &cobra.Command{
	Use:   config.TranslateInLang(deployCmdUses, config.UILanguage),
	Short: config.TranslateInLang(deployCmdShorts, config.UILanguage),
}

func init() {
	contractDeployCmd.AddCommand(contractDeployBytecodeCmd)
	contractDeployCmd.AddCommand(contractDeployBinCmd)
	contractDeployCmd.AddCommand(contractDeploySolCmd)
}

func decodeBytecode(bytecode string) ([]byte, error) {
	return hex.DecodeString(util.TrimHexPrefix(bytecode))
}
