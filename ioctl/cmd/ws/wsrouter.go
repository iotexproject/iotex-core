package ws

import (
	"bytes"
	_ "embed" // used to embed contract abi

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
)

var wsRouterCmd = &cobra.Command{
	Use: "router",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "w3bstream project routing to Dapp",
		config.Chinese: "w3bstream 项目路由到Dapp合约",
	}, config.UILanguage),
}

var (
	//go:embed contracts/abis/W3bstreamRouter.json
	routerJSON    []byte
	routerAddress string
	routerABI     abi.ABI
)

const (
	funcBindDapp   = "bindDapp"
	funcUnbindDapp = "unbindDapp"
)

const (
	eventDappBound   = "DappBound"
	eventDappUnbound = "DappUnbound"
)

func init() {
	var err error
	routerABI, err = abi.JSON(bytes.NewReader(routerJSON))
	if err != nil {
		panic(err)
	}
	routerAddress = config.ReadConfig.WsRouterContract

	WsCmd.AddCommand(wsRouterCmd)
}
