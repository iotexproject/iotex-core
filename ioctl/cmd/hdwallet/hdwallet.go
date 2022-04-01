// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	_hdwalletCmdShorts = map[config.Language]string{
		config.English: "Manage hdwallets of IoTeX blockchain",
		config.Chinese: "管理IoTeX区块链上的钱包",
	}
	_hdwalletCmdUses = map[config.Language]string{
		config.English: "hdwallet",
		config.Chinese: "钱包",
	}
)

// Errors
var (
	ErrPasswdNotMatch = errors.New("password doesn't match")
)

// HdwalletCmd represents the hdwallet command
var HdwalletCmd = &cobra.Command{
	Use:   config.TranslateInLang(_hdwalletCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_hdwalletCmdShorts, config.UILanguage),
}

// DefaultRootDerivationPath for iotex
// https://github.com/satoshilabs/slips/blob/master/slip-0044.md
const DefaultRootDerivationPath = "m/44'/304'"

var _hdWalletConfigFile = config.ReadConfig.Wallet + "/hdwallet"

func init() {
	HdwalletCmd.AddCommand(_hdwalletCreateCmd)
	HdwalletCmd.AddCommand(_hdwalletDeleteCmd)
	HdwalletCmd.AddCommand(_hdwalletImportCmd)
	HdwalletCmd.AddCommand(_hdwalletExportCmd)
	HdwalletCmd.AddCommand(_hdwalletDeriveCmd)
}
