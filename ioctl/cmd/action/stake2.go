// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	stake2CmdUses = map[config.Language]string{
		config.English: "stake2",
		config.Chinese: "stake2",
	}
	stake2CmdShorts = map[config.Language]string{
		config.English: "support native staking from ioctl",
		config.Chinese: "支持来自ioctl的本地质押",
	}
	stake2FlagEndpointUsages = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置所有端点",
	}
	stake2FlagInsecureUsages = map[config.Language]string{
		config.English: "insecure connection for once (default false)",
		config.Chinese: "一次不安全的连接（默认为false)",
	}
)

var stake2AutoRestake bool

//Stake2Cmd represent stake2 command
var Stake2Cmd = &cobra.Command{
	Use:   config.TranslateInLang(stake2CmdUses, config.UILanguage),
	Short: config.TranslateInLang(stake2CmdShorts, config.UILanguage),
}

func init() {
	Stake2Cmd.AddCommand(stake2CreateCmd)
	Stake2Cmd.AddCommand(stake2RegisterCmd)
	Stake2Cmd.AddCommand(stake2ChangeCmd)
	Stake2Cmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint", config.ReadConfig.Endpoint, config.TranslateInLang(stake2FlagEndpointUsages, config.UILanguage))
	Stake2Cmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure, config.TranslateInLang(stake2FlagInsecureUsages, config.UILanguage))
}
