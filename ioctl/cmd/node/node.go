// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package node

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/flag"
)

// Multi-language support
var (
	_nodeCmdUses = map[config.Language]string{
		config.English: "node",
		config.Chinese: "node",
	}
	_nodeCmdShorts = map[config.Language]string{
		config.English: "Deal with nodes of IoTeX blockchain",
		config.Chinese: "处理IoTeX区块链的节点",
	}
	_flagEndpointUsages = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点",
	}
	_flagInsecureUsages = map[config.Language]string{
		config.English: "insecure connection for once",
		config.Chinese: "一次不安全的连接",
	}
	_allFlag = flag.BoolVarP("all", "a", false, "returns all delegates")
)

// NodeCmd represents the node command
var NodeCmd = &cobra.Command{
	Use:   config.TranslateInLang(_nodeCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_nodeCmdShorts, config.UILanguage),
}

func init() {
	NodeCmd.AddCommand(_nodeDelegateCmd)
	NodeCmd.AddCommand(_nodeRewardCmd)
	NodeCmd.AddCommand(_nodeProbationlistCmd)
	NodeCmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, config.TranslateInLang(_flagEndpointUsages, config.UILanguage))
	NodeCmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure,
		config.TranslateInLang(_flagInsecureUsages, config.UILanguage))
	_allFlag.RegisterCommand(_nodeDelegateCmd)
}
