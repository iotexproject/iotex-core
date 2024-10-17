package ws

import (
	"bytes"
	_ "embed" // used to embed contract abi

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
)

var (
	_flagDevicesUsages = map[config.Language]string{
		config.English: "device address for the project, separate multiple addresses with commas",
		config.Chinese: "该project的设备地址，多个地址用逗号分割",
	}
)

var wsProjectDeviceCmd = &cobra.Command{
	Use: "device",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "w3bstream device management",
		config.Chinese: "w3bstream device 节点管理",
	}, config.UILanguage),
}

var (
	//go:embed contracts/abis/ProjectDevice.json
	projectDeviceJSON    []byte
	projectDeviceAddress string
	projectDeviceABI     abi.ABI
)

const (
	funcDevicesApprove   = "approve"
	funcDevicesUnapprove = "unapprove"
	funcDevicesApproved  = "approved"
)

const (
	eventOnApprove   = "Approve"
	eventOnUnapprove = "Unapprove"
)

func init() {
	var err error
	projectDeviceABI, err = abi.JSON(bytes.NewReader(projectDeviceJSON))
	if err != nil {
		panic(err)
	}
	projectDeviceAddress = config.ReadConfig.WsProjectDevicesContract

	WsCmd.AddCommand(wsProjectDeviceCmd)
}
