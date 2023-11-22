package znode

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
)

var (
	// ZnodeCmd represents the znode command
	ZnodeCmd = &cobra.Command{
		Use:   "znode",
		Short: config.TranslateInLang(znodeCmdShorts, config.UILanguage),
	}

	// znodeCmdShorts command multi-lang supports
	znodeCmdShorts = map[config.Language]string{
		config.English: "Znode operations",
		config.Chinese: "Znode 操作",
	}

	//
	_flagChainEndpointUsages = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点",
	}
	_flagZnodeEndpointUsages = map[config.Language]string{
		config.English: "set znode endpoint for once",
		config.Chinese: "一次设置znode端点",
	}
	_flagZnodeContractAddressUsages = map[config.Language]string{
		config.English: "set znode contract address for once",
		config.Chinese: "一次设置znode合约地址",
	}
)

func init() {
	ZnodeCmd.AddCommand(znodeMessage)
	ZnodeCmd.AddCommand(znodeCode)

	ZnodeCmd.PersistentFlags().StringVar(
		&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, config.TranslateInLang(_flagChainEndpointUsages, config.UILanguage),
	)
	ZnodeCmd.PersistentFlags().StringVar(
		&config.ReadConfig.ZnodeEndpoint, "znode-endpoint",
		config.ReadConfig.ZnodeEndpoint, config.TranslateInLang(_flagZnodeEndpointUsages, config.UILanguage),
	)
	ZnodeCmd.PersistentFlags().StringVar(
		&config.ReadConfig.ZnodeContractAddress, "znode-contract-address",
		config.ReadConfig.ZnodeContractAddress, config.TranslateInLang(_flagZnodeContractAddressUsages, config.UILanguage),
	)
}
