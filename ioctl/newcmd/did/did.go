// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package did

import (
	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/spf13/cobra"
)

// Multi-language support
var (
	_dIDCmdShorts = map[config.Language]string{
		config.English: "DID command",
		config.Chinese: "DID command",
	}
)

// NewDidCmd represents the did command
func NewDidCmd(client ioctl.Client) *cobra.Command {
	short, _ := client.SelectTranslation(_dIDCmdShorts)

	cmd := &cobra.Command{
		Use:   "did",
		Short: short,
	}

	client.SetEndpointWithFlag(cmd.PersistentFlags().StringVar)
	client.SetInsecureWithFlag(cmd.PersistentFlags().BoolVar)
	return cmd
}
