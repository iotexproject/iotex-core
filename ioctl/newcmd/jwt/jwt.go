// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package jwt

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	_jwtCmdShorts = map[config.Language]string{
		config.English: "Manage Json Web Token on IoTeX blockchain",
		config.Chinese: "管理IoTeX区块链上的JWT",
	}
)

// NewJwtCmd represents the jwt command
func NewJwtCmd(client ioctl.Client) *cobra.Command {
	short, _ := client.SelectTranslation(_jwtCmdShorts)
	cmd := &cobra.Command{
		Use:   "jwt",
		Short: short,
	}
	cmd.AddCommand(NewJwtSignCmd(client))
	return cmd
}
