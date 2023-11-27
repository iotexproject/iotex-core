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

	// wsCmdShorts command multi-lang supports
	wsCmdShorts = map[config.Language]string{
		config.English: "W3bstream node operations",
		config.Chinese: "W3bstream节点操作",
	}

	_flagChainEndpointUsages = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点",
	}

	_flagWsEndpointUsages = map[config.Language]string{
		config.English: "set w3bsteram endpoint for once",
		config.Chinese: "一次设置w3bstream端点",
	}
)

func init() {
	WsCmd.AddCommand(wsMessage)

	WsCmd.PersistentFlags().StringVar(
		&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, config.TranslateInLang(_flagChainEndpointUsages, config.UILanguage),
	)
	WsCmd.PersistentFlags().StringVar(
		&config.ReadConfig.WsEndpoint, "ws-endpoint",
		config.ReadConfig.WsEndpoint, config.TranslateInLang(_flagWsEndpointUsages, config.UILanguage),
	)
}
