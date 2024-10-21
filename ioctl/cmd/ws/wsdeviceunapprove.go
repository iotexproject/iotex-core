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
	// wsProjectDeviceUnapprove represents unapprove device to send message to w3bstream project command
	wsProjectDeviceUnapprove = &cobra.Command{
		Use:   "unapprove",
		Short: config.TranslateInLang(wsProjectDeviceUnapproveShorts, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cmd.Flags().GetUint64("project-id")
			if err != nil {
				return errors.Wrap(err, "failed to get flag project-id")
			}
			devices, err := cmd.Flags().GetString("devices")
			if err != nil {
				return errors.Wrap(err, "failed to get flag devices")
			}

			out, err := unapproveProjectDevice(projectID, devices)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(out)
			return nil
		},
	}

	wsProjectDeviceUnapproveShorts = map[config.Language]string{
		config.English: "unapprove devices for w3bstream project",
		config.Chinese: "取消授权w3bstream项目的设备",
	}
)

func init() {
	wsProjectDeviceUnapprove.Flags().Uint64P("project-id", "", 0, config.TranslateInLang(_flagProjectIDUsages, config.UILanguage))
	wsProjectDeviceUnapprove.Flags().StringP("devices", "", "", config.TranslateInLang(_flagDevicesUsages, config.UILanguage))

	_ = wsProjectDeviceUnapprove.MarkFlagRequired("project-id")
	_ = wsProjectDeviceUnapprove.MarkFlagRequired("devices")

	wsProjectDeviceCmd.AddCommand(wsProjectDeviceUnapprove)
}

func unapproveProjectDevice(projectID uint64, devices string) (string, error) {
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

	value := new(contracts.ProjectDeviceUnapprove)
	result := NewContractResult(&projectDeviceABI, eventOnUnapprove, value)
	if _, err = caller.CallAndRetrieveResult(funcDevicesUnapprove, []any{
		big.NewInt(int64(projectID)),
		deviceAddress,
	}, result); err != nil {
		return "", errors.Wrap(err, "failed to read contract")
	}
	if _, err = result.Result(); err != nil {
		return "", err
	}

	return fmt.Sprintf("unapprove %d device", len(deviceAddress)), nil
}
