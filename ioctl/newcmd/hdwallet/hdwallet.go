// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Errors
var (
	ErrPasswdNotMatch = errors.New("password doesn't match")
)

// DefaultRootDerivationPath for iotex
// https://github.com/satoshilabs/slips/blob/master/slip-0044.md
const DefaultRootDerivationPath = "m/44'/304'"

// Multi-language support
var (
	_hdwalletCmdShorts = map[config.Language]string{
		config.English: "Manage hdwallets of IoTeX blockchain",
		config.Chinese: "管理IoTeX区块链上的钱包",
	}
)

// NewHdwalletCmd represents the new hdwallet command.
func NewHdwalletCmd(client ioctl.Client) *cobra.Command {
	hdwalletShorts, _ := client.SelectTranslation(_hdwalletCmdShorts)
	cmd := &cobra.Command{
		Use:   "hdwallet",
		Short: hdwalletShorts,
	}
	cmd.AddCommand(NewHdwalletCreateCmd(client))
	cmd.AddCommand(NewHdwalletDeleteCmd(client))
	cmd.AddCommand(NewHdwalletDeriveCmd(client))
	cmd.AddCommand(NewHdwalletExportCmd(client))
	cmd.AddCommand(NewHdwalletImportCmd(client))
	return cmd
}
