package ws

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/flag"
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
		config.English: "set w3bstream endpoint for once",
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

	_flagProjectRegisterContractAddressUsages = map[config.Language]string{
		config.English: "set w3bstream project register contract address for once",
		config.Chinese: "一次设置w3bstream项目注册合约地址",
	}

	_flagProjectStoreContractAddressUsages = map[config.Language]string{
		config.English: "set w3bstream project store contract address for once",
		config.Chinese: "一次设置w3bstream项目存储合约地址",
	}

	_flagFleetManagementContractAddressUsages = map[config.Language]string{
		config.English: "set w3bstream fleet management contract address for once",
		config.Chinese: "一次设置w3bstream项目管理合约地址",
	}

	_flagProverStoreContractAddressUsages = map[config.Language]string{
		config.English: "set w3bstream prover store contract address for once",
		config.Chinese: "一次设置w3bstream prover存储合约地址",
	}
	_flagTransferAmountUsages = map[config.Language]string{
		config.English: "amount(rau) need to pay, default 0",
		config.Chinese: "需要支付的token数量(单位rau), 默认0",
	}

	_flagProjectDevicesContractAddressUsages = map[config.Language]string{
		config.English: "set w3bstream project devices contract address for once",
		config.Chinese: "一次设置w3bstream project设备合约地址",
	}
	_flagProjectIDUsages = map[config.Language]string{
		config.English: "project id",
		config.Chinese: "项目ID",
	}

	_flagRouterContractAddressUsages = map[config.Language]string{
		config.English: "set w3bstream router contract address for once",
		config.Chinese: "一次设置w3bstream router合约地址",
	}

	_flagVmTypeContractAddressUsages = map[config.Language]string{
		config.English: "set w3bstream vmType contract address for once",
		config.Chinese: "一次设置w3bstream vmType合约地址",
	}
)

var (
	// transferAmount
	transferAmount = flag.NewStringVarP("amount", "", "0", config.TranslateInLang(_flagTransferAmountUsages, config.UILanguage))
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
		&config.ReadConfig.WsProjectRegisterContract, "project-register-contract",
		config.ReadConfig.WsProjectRegisterContract, config.TranslateInLang(_flagProjectRegisterContractAddressUsages, config.UILanguage),
	)
	WsCmd.PersistentFlags().StringVar(
		&config.ReadConfig.WsProjectStoreContract, "project-store-contract",
		config.ReadConfig.WsProjectStoreContract, config.TranslateInLang(_flagProjectStoreContractAddressUsages, config.UILanguage),
	)
	WsCmd.PersistentFlags().StringVar(
		&config.ReadConfig.WsFleetManagementContract, "fleet-management-contract",
		config.ReadConfig.WsFleetManagementContract, config.TranslateInLang(_flagFleetManagementContractAddressUsages, config.UILanguage),
	)
	WsCmd.PersistentFlags().StringVar(
		&config.ReadConfig.WsProverStoreContract, "prover-store-contract",
		config.ReadConfig.WsProverStoreContract, config.TranslateInLang(_flagProverStoreContractAddressUsages, config.UILanguage),
	)
	WsCmd.PersistentFlags().StringVar(
		&config.ReadConfig.WsProjectDevicesContract, "project-devices-contract",
		config.ReadConfig.WsProjectDevicesContract, config.TranslateInLang(_flagProjectDevicesContractAddressUsages, config.UILanguage),
	)
	WsCmd.PersistentFlags().StringVar(
		&config.ReadConfig.WsRouterContract, "router-contract",
		config.ReadConfig.WsRouterContract, config.TranslateInLang(_flagRouterContractAddressUsages, config.UILanguage),
	)
	WsCmd.PersistentFlags().StringVar(
		&config.ReadConfig.WsVmTypeContract, "vmType-contract",
		config.ReadConfig.WsVmTypeContract, config.TranslateInLang(_flagVmTypeContractAddressUsages, config.UILanguage),
	)
}
