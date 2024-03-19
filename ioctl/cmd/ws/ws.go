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
)

// multi-language flags descriptions
var (
	_flagChainEndpointUsages = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点",
	}
	_flagWsEndpointUsages = map[config.Language]string{
		config.English: "set w3bsteram endpoint for once",
		config.Chinese: "一次设置w3bstream端点",
	}
	_flagProjectIDUsages = map[config.Language]string{
		config.English: "project id",
		config.Chinese: "项目ID",
	}
	_flagProjectVersionUsages = map[config.Language]string{
		config.English: "project version",
		config.Chinese: "项目版本",
	}
	_flagSendDataUsages = map[config.Language]string{
		config.English: "send data",
		config.Chinese: "要发送的数据",
	}
	_flagVMTypeUsages = map[config.Language]string{
		config.English: "vm type, support risc0, halo2",
		config.Chinese: "虚拟机类型，目前支持risc0和halo2",
	}
	_flagCodeFileUsages = map[config.Language]string{
		config.English: "code file",
		config.Chinese: "代码文件",
	}
	_flagConfFileUsages = map[config.Language]string{
		config.English: "conf file",
		config.Chinese: "配置文件",
	}
	_flagExpandParamUsages = map[config.Language]string{
		config.English: "expand param, if you use risc0 vm, need it.",
		config.Chinese: "扩展参数，risc0虚拟机需要此参数",
	}
	_flagMessageIDUsages = map[config.Language]string{
		config.English: "message id",
		config.Chinese: "消息ID",
	}
	_flagProjectOperatorUsages = map[config.Language]string{
		config.English: "project operator",
		config.Chinese: "项目操作者账户地址",
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
	_flagProjectConfigFileUsages = map[config.Language]string{
		config.English: "project config file path",
		config.Chinese: "项目配置文件路径",
	}
	_flagProjectConfigHashUsages = map[config.Language]string{
		config.English: "project config file hash(sha256) for validating",
		config.Chinese: "项目配置文件sha256哈希",
	}
	_flagVersionUsages = map[config.Language]string{
		config.English: "version for the project config",
		config.Chinese: "该project config的版本号",
	}
)

func init() {
	WsCmd.AddCommand(wsMessage)
	WsCmd.AddCommand(wsCode)
	WsCmd.AddCommand(wsProject)

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
