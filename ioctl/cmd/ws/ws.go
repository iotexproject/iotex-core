package ws

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
)

var (
	// WsCmd represents the w3bstream command
	WsCmd = &cobra.Command{
		Use:   "ws",
		Short: config.TranslateInLang(wsCmdShorts, config.UILanguage),
	}

	wsCmdShorts = map[config.Language]string{
		config.English: "W3bstream node operations",
		config.Chinese: "W3bstream节点操作",
	}
)

// flags multi-language
var (
	_flagChainEndpointUsages = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点",
	}

	_flagWsEndpointUsages = map[config.Language]string{
		config.English: "set w3bsteram endpoint for once",
		config.Chinese: "一次设置w3bstream端点",
	}

	_flagIPFSEndpointUsages = map[config.Language]string{
		config.English: "set ipfs endpoint for resource uploading for once",
		config.Chinese: "一次设置ipfs端点",
	}

	_flagIPFSGatewayUsages = map[config.Language]string{
		config.English: "set ipfs gateway for resource fetching for once",
		config.Chinese: "一次设置ipfs网关",
	}

	_flagContractAddressUsages = map[config.Language]string{
		config.English: "set w3bsteram project register contract address for once",
		config.Chinese: "一次设置w3bstream项目注册合约地址",
	}
)

func init() {
	WsCmd.PersistentFlags().StringVar(
		&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, config.TranslateInLang(_flagChainEndpointUsages, config.UILanguage),
	)
	WsCmd.PersistentFlags().StringVar(
		&config.ReadConfig.WsEndpoint, "ws-endpoint",
		config.ReadConfig.WsEndpoint, config.TranslateInLang(_flagWsEndpointUsages, config.UILanguage),
	)
	WsCmd.PersistentFlags().StringVar(
		&config.ReadConfig.IPFSEndpoint, "ipfs-endpoint",
		config.ReadConfig.IPFSEndpoint, config.TranslateInLang(_flagIPFSEndpointUsages, config.UILanguage),
	)
	WsCmd.PersistentFlags().StringVar(
		&config.ReadConfig.IPFSGateway, "ipfs-gateway",
		config.ReadConfig.IPFSGateway, config.TranslateInLang(_flagIPFSGatewayUsages, config.UILanguage),
	)
	WsCmd.PersistentFlags().StringVar(
		&config.ReadConfig.WsRegisterContract, "contract-address",
		config.ReadConfig.WsRegisterContract, config.TranslateInLang(_flagContractAddressUsages, config.UILanguage),
	)
}
