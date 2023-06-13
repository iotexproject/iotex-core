// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package did

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	DIDCmdShorts = map[config.Language]string{
		config.English: "DID command",
		config.Chinese: "DID command",
	}
	_flagEndpoint = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点",
	}
	_flagInsecure = map[config.Language]string{
		config.English: "insecure connection for once",
		config.Chinese: "一次不安全连接",
	}
)

// DIDCmd represents the DID command
var DIDCmd = &cobra.Command{
	Use:   "did",
	Short: config.TranslateInLang(DIDCmdShorts, config.UILanguage),
}

func init() {
	DIDCmd.AddCommand(_didGenerateCmd)
	DIDCmd.AddCommand(_didRegisterCmd)
	DIDCmd.AddCommand(_didGetCmd)
	DIDCmd.AddCommand(_didDeregisterCmd)
	DIDCmd.AddCommand(_didServiceAddCmd)
	DIDCmd.AddCommand(_didServiceRemoveCmd)
	DIDCmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, config.TranslateInLang(_flagEndpoint, config.UILanguage))
	DIDCmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure, config.TranslateInLang(_flagInsecure, config.UILanguage))
}
