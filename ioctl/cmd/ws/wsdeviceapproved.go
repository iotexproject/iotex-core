package ws

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
)

var (
	// wsProjectDeviceApproved represents unapprove device to send message to w3bstream project command
	wsProjectDeviceApproved = &cobra.Command{
		Use:   "approved",
		Short: config.TranslateInLang(wsProjectDeviceApprovedShorts, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cmd.Flags().GetUint64("project-id")
			if err != nil {
				return errors.Wrap(err, "failed to get flag project-id")
			}
			device, err := cmd.Flags().GetString("device")
			if err != nil {
				return errors.Wrap(err, "failed to get flag devices")
			}

			out, err := approvedProjectDevice(projectID, device)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(output.JSONString(out))
			return nil
		},
	}

	wsProjectDeviceApprovedShorts = map[config.Language]string{
		config.English: "approved devices for w3bstream project",
		config.Chinese: "查询是否授权w3bstream项目的设备",
	}

	_flagDeviceUsages = map[config.Language]string{
		config.English: "device address for the project",
		config.Chinese: "该project的设备地址",
	}
)

func init() {
	wsProjectDeviceApproved.Flags().Uint64P("project-id", "", 0, config.TranslateInLang(_flagProjectIDUsages, config.UILanguage))
	wsProjectDeviceApproved.Flags().StringP("device", "", "", config.TranslateInLang(_flagDeviceUsages, config.UILanguage))

	_ = wsProjectDeviceApproved.MarkFlagRequired("project-id")
	_ = wsProjectDeviceApproved.MarkFlagRequired("device")

	wsProjectDeviceCmd.AddCommand(wsProjectDeviceApproved)
}

func approvedProjectDevice(projectID uint64, device string) (any, error) {
	addr, err := address.FromString(device)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid device address: %s", device)
	}
	deviceAddress := common.BytesToAddress(addr.Bytes())

	caller, err := NewContractCaller(projectDeviceABI, projectDeviceAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create contract caller")
	}

	result := NewContractResult(&projectDeviceABI, funcDevicesApproved, new(bool))
	if err = caller.Read(funcDevicesApproved, []any{
		big.NewInt(int64(projectID)),
		deviceAddress,
	}, result); err != nil {
		return nil, errors.Wrap(err, "failed to read contract")
	}
	approved, err := result.Result()
	if err != nil {
		return nil, err
	}

	return &struct {
		ProjectID uint64 `json:"projectID"`
		Device    string `json:"device"`
		Approved  bool   `json:"approved"`
	}{
		ProjectID: projectID,
		Device:    device,
		Approved:  *(approved.(*bool)),
	}, nil
}
