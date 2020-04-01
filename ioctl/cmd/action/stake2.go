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
)

var stake2AutoRestake bool

//Stake2Cmd represent stake2 command
var Stake2Cmd = &cobra.Command{
	Use: config.TranslateInLang(stake2CmdUses, config.UILanguage),
}

func init() {
	Stake2Cmd.AddCommand(stake2RgisterCmd)
	Stake2Cmd.AddCommand(stake2CreateCmd)
}
