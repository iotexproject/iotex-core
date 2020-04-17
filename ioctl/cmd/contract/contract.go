// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
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
)

// ContractCmd represents the contract command
var ContractCmd = &cobra.Command{
	Use:   config.TranslateInLang(contractCmdUses, config.UILanguage),
	Short: config.TranslateInLang(contractCmdShorts, config.UILanguage),
}

func init() {
	ContractCmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, config.TranslateInLang(flagEndpointUsages, config.UILanguage))
	ContractCmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure,
		config.TranslateInLang(flagInsecureUsages, config.UILanguage))
}
