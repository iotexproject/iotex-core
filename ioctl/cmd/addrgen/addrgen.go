// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// Package addrgen provides commands for generating cryptographic keys
package addrgen

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
)

// Multi-language support
var (
	_addrgenCmdShorts = map[config.Language]string{
		config.English: "Generate cryptographic keys for IoTeX blockchain",
		config.Chinese: "生成IoTeX区块链的加密密钥",
	}
	_addrgenCmdLongs = map[config.Language]string{
		config.English: `Generate ECDSA and BLS private/public key pairs for IoTeX blockchain usage.`,
		config.Chinese: `生成用于IoTeX区块链的ECDSA和BLS私钥/公钥对。`,
	}
)

// AddrgenCmd represents the addrgen command
var AddrgenCmd = &cobra.Command{
	Use:   "addrgen",
	Short: config.TranslateInLang(_addrgenCmdShorts, config.UILanguage),
	Long:  config.TranslateInLang(_addrgenCmdLongs, config.UILanguage),
}

func init() {
	AddrgenCmd.AddCommand(_blsCmd)
	// TODO: Add ECDSA command when implementing ECDSA key generation
}
