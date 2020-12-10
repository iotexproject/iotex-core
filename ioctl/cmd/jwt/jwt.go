// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package jwt

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/flag"
)

// Multi-language support
var (
	jwtCmdShorts = map[config.Language]string{
		config.English: "Manage Json Web Token on IoTeX blockchain",
		config.Chinese: "管理IoTeX区块链上的JWT",
	}
	jwtCmdUses = map[config.Language]string{
		config.English: "jwt",
		config.Chinese: "json网络令牌",
	}
)

// JwtCmd represents the jwt command
var JwtCmd = &cobra.Command{
	Use:   config.TranslateInLang(jwtCmdUses, config.UILanguage),
	Short: config.TranslateInLang(jwtCmdShorts, config.UILanguage),
}

func init() {
	JwtCmd.AddCommand(jwtSignCmd)
	action.RegisterWriteCommand(jwtSignCmd)
	flag.WithArgumentsFlag.RegisterCommand(jwtSignCmd)
}
