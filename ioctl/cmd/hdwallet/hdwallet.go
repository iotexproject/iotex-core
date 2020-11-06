// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"crypto/ecdsa"
	"errors"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	hdwalletCmdShorts = map[config.Language]string{
		config.English: "Manage hdwallets of IoTeX blockchain",
		config.Chinese: "管理IoTeX区块链上的钱包",
	}
	hdwalletCmdUses = map[config.Language]string{
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
	Use:   config.TranslateInLang(hdwalletCmdUses, config.UILanguage),
	Short: config.TranslateInLang(hdwalletCmdShorts, config.UILanguage),
}

// DefaultRootDerivationPath for iotex
// https://github.com/satoshilabs/slips/blob/master/slip-0044.md
const DefaultRootDerivationPath = "m/44'/304'/0'/0"

var hdWalletConfigFile = config.ReadConfig.Wallet + "/hdwallet"

func init() {
	HdwalletCmd.AddCommand(hdwalletCreateCmd)
	HdwalletCmd.AddCommand(hdwalletDeleteCmd)
	HdwalletCmd.AddCommand(hdwalletImportCmd)
	HdwalletCmd.AddCommand(hdwalletExportCmd)
	HdwalletCmd.AddCommand(hdwalletUseCmd)
}

func hashECDSAPublicKey(publicKey *ecdsa.PublicKey) []byte {
	k := crypto.FromECDSAPub(publicKey)
	h := hash.Hash160b(k[1:])
	return h[:]
}
