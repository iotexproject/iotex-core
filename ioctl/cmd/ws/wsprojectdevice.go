package ws

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/flag"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

var (
	wsProjectDevicesCmd = &cobra.Command{
		Use: "devices",
		Short: config.TranslateInLang(map[config.Language]string{
			config.English: "project devices management",
			config.Chinese: "项目设备配置",
		}, config.UILanguage),
	}

	wsProjectDevicesApproveCmd = &cobra.Command{
		Use: "approve",
		Short: config.TranslateInLang(map[config.Language]string{
			config.English: "approve devices",
			config.Chinese: "授权",
		}, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := approveDevices(
				big.NewInt(int64(projectID.Value().(uint64))),
				projectAttrKey.Value().(string),
			)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(output.JSONString(out))
			return nil
		},
	}

	wsProjectDevicesUnapproveCmd = &cobra.Command{
		Use: "unapprove",
		Short: config.TranslateInLang(map[config.Language]string{
			config.English: "unapprove devices",
			config.Chinese: "取消授权",
		}, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := unapproveDevices(
				big.NewInt(int64(projectID.Value().(uint64))),
				projectAttrKey.Value().(string),
			)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(output.JSONString(out))
			return nil
		},
	}

	_flagprojectDevicesUsage = map[config.Language]string{
		config.English: "project devices",
		config.Chinese: "项目设备",
	}

	projectDevices = flag.NewStringVarP("devices", "", "", config.TranslateInLang(_flagprojectDevicesUsage, config.UILanguage))

	projectDevicesABI = `[
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "_projectId",
				"type": "uint256"
			},
			{
				"internalType": "address[]",
				"name": "_devices",
				"type": "address[]"
			}
		],
		"name": "approve",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "_projectId",
				"type": "uint256"
			},
			{
				"internalType": "address[]",
				"name": "_devices",
				"type": "address[]"
			}
		],
		"name": "unapprove",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	}]`
)

func init() {
	projectID.RegisterCommand(wsProjectDevicesApproveCmd)
	projectID.MarkFlagRequired(wsProjectDevicesApproveCmd)
	projectDevices.RegisterCommand(wsProjectDevicesApproveCmd)
	projectDevices.MarkFlagRequired(wsProjectDevicesApproveCmd)

	projectID.RegisterCommand(wsProjectDevicesUnapproveCmd)
	projectID.MarkFlagRequired(wsProjectDevicesUnapproveCmd)
	projectDevices.RegisterCommand(wsProjectDevicesUnapproveCmd)
	projectDevices.MarkFlagRequired(wsProjectDevicesUnapproveCmd)

	wsProjectDevicesCmd.AddCommand(wsProjectDevicesApproveCmd)
	wsProjectDevicesCmd.AddCommand(wsProjectDevicesUnapproveCmd)

	wsProject.AddCommand(wsProjectDevicesCmd)
}

func approveDevices(projectID *big.Int, devices string) (any, error) {
	projectDevices, err := address.FromHex(config.ReadConfig.WsProjectDevicesContract)
	if err != nil {
		return nil, output.NewError(output.AddressError, "failed to convert project devices address", err)
	}
	projectDevicesAbi, err := abi.JSON(strings.NewReader(projectDevicesABI))
	if err != nil {
		return nil, output.NewError(output.SerializationError, "failed to unmarshal abi", err)
	}

	devicesS := strings.Split(devices, ",")
	devicesA := make([]common.Address, len(devicesS))
	for i := 0; i < len(devicesS); i++ {
		devicesA[i] = common.HexToAddress(devicesS[i])
	}

	data, err := projectDevicesAbi.Pack("approve", projectID, devicesA)
	if err != nil {
		return nil, output.NewError(output.ConvertError, "failed to pack approve arguments", err)
	}
	res, err := action.ExecuteAndResponse(projectDevices.String(), big.NewInt(0), data)
	if err != nil {
		return nil, output.NewError(output.ConvertError, "send action failure", err)
	}
	return "Apporve action hash: " + res.ActionHash, nil
}

func unapproveDevices(projectID *big.Int, devices string) (any, error) {
	projectDevices, err := address.FromHex(config.ReadConfig.WsProjectDevicesContract)
	if err != nil {
		return nil, output.NewError(output.AddressError, "failed to convert project devices address", err)
	}
	projectDevicesAbi, err := abi.JSON(strings.NewReader(projectDevicesABI))
	if err != nil {
		return nil, output.NewError(output.SerializationError, "failed to unmarshal abi", err)
	}

	devicesS := strings.Split(devices, ",")
	devicesA := make([]common.Address, len(devicesS))
	for i := 0; i < len(devicesS); i++ {
		devicesA[i] = common.HexToAddress(devicesS[i])
	}

	data, err := projectDevicesAbi.Pack("unapprove", projectID, devicesA)
	if err != nil {
		return nil, output.NewError(output.ConvertError, "failed to pack unapprove arguments", err)
	}
	res, err := action.ExecuteAndResponse(projectDevices.String(), big.NewInt(0), data)
	if err != nil {
		return nil, output.NewError(output.ConvertError, "send action failure", err)
	}
	return "Unapporve action hash: " + res.ActionHash, nil
}
