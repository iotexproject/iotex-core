package ws

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/ws/contracts"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
)

var (
	// wsProjectDeviceApprove represents approve device to send message to w3bstream project command
	wsProjectDeviceApprove = &cobra.Command{
		Use:   "approve",
		Short: config.TranslateInLang(wsProjectDeviceApproveShorts, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cmd.Flags().GetUint64("project-id")
			if err != nil {
				return errors.Wrap(err, "failed to get flag project-id")
			}
			devices, err := cmd.Flags().GetString("devices")
			if err != nil {
				return errors.Wrap(err, "failed to get flag devices")
			}

			out, err := approveProjectDevice(projectID, devices)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(out)
			return nil
		},
	}

	wsProjectDeviceApproveShorts = map[config.Language]string{
		config.English: "approve devices for w3bstream project",
		config.Chinese: "授权w3bstream项目的设备",
	}
)

func init() {
	wsProjectDeviceApprove.Flags().Uint64P("project-id", "", 0, config.TranslateInLang(_flagProjectIDUsages, config.UILanguage))
	wsProjectDeviceApprove.Flags().StringP("devices", "", "", config.TranslateInLang(_flagDevicesUsages, config.UILanguage))

	_ = wsProjectDeviceApprove.MarkFlagRequired("project-id")
	_ = wsProjectDeviceApprove.MarkFlagRequired("devices")

	wsProjectDeviceCmd.AddCommand(wsProjectDeviceApprove)
}

func approveProjectDevice(projectID uint64, devices string) (string, error) {
	deviceArr := []string{devices}
	if strings.Contains(devices, ",") {
		deviceArr = strings.Split(devices, ",")
	}

	deviceAddress := make([]common.Address, 0, len(deviceArr))
	for _, device := range deviceArr {
		addr, err := address.FromString(device)
		if err != nil {
			return "", errors.Wrapf(err, "invalid device address: %s", device)
		}
		deviceAddress = append(deviceAddress, common.BytesToAddress(addr.Bytes()))
	}

	caller, err := NewContractCaller(projectDeviceABI, projectDeviceAddress)
	if err != nil {
		return "", errors.Wrap(err, "failed to create contract caller")
	}

	value := new(contracts.ProjectDeviceApprove)
	result := NewContractResult(&projectDeviceABI, eventOnApprove, value)
	if _, err = caller.CallAndRetrieveResult(funcDevicesApprove, []any{
		big.NewInt(int64(projectID)),
		deviceAddress,
	}, result); err != nil {
		return "", errors.Wrap(err, "failed to read contract")
	}
	if _, err = result.Result(); err != nil {
		return "", err
	}

	return fmt.Sprintf("approve %d device", len(deviceAddress)), nil
}
