package ioid

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/spf13/cobra"
)

// Multi-language support
var (
	_projectUsages = map[config.Language]string{
		config.English: "project",
		config.Chinese: "project",
	}
	_projectShorts = map[config.Language]string{
		config.English: "Project detail",
		config.Chinese: "项目详情",
	}
)

// _projectCmd represents the ioID apply command
var _projectCmd = &cobra.Command{
	Use:   config.TranslateInLang(_projectUsages, config.UILanguage),
	Short: config.TranslateInLang(_projectShorts, config.UILanguage),
	Args:  cobra.MinimumNArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		err := project()
		return output.PrintError(err)
	},
}

func init() {
	_projectCmd.Flags().StringVarP(
		&ioIDStore, "ioIDStore", "i",
		"0xA0C9f9A884cdAE649a42F16b057735Bc4fE786CD",
		config.TranslateInLang(_ioIDStoreUsages, config.UILanguage),
	)
	_projectCmd.Flags().Uint64VarP(
		&projectId, "projectId", "p", 0,
		config.TranslateInLang(_projectIdUsages, config.UILanguage),
	)
}

func project() error {
	ioioIDStore, err := address.FromHex(ioIDStore)
	if err != nil {
		return output.NewError(output.AddressError, "failed to convert ioIDStore address", err)
	}

	ioIDStoreAbi, err := abi.JSON(strings.NewReader(ioIDStoreABI))
	if err != nil {
		return output.NewError(output.SerializationError, "failed to unmarshal abi", err)
	}

	data, err := ioIDStoreAbi.Pack("project")
	if err != nil {
		return output.NewError(output.ConvertError, "failed to pack project arguments", err)
	}
	res, err := action.Read(ioioIDStore, "0", data)
	if err != nil {
		return output.NewError(output.APIError, "failed to read contract", err)
	}
	data, _ = hex.DecodeString(res)
	projectAddr, err := ioIDStoreAbi.Unpack("project", data)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to unpack project response", err)
	}

	data, err = ioIDStoreAbi.Pack("projectDeviceContract", new(big.Int).SetUint64(projectId))
	if err != nil {
		return output.NewError(output.ConvertError, "failed to pack project device contract arguments", err)
	}
	res, err = action.Read(ioioIDStore, "0", data)
	if err != nil {
		return output.NewError(output.APIError, "failed to read contract", err)
	}
	data, _ = hex.DecodeString(res)
	deviceContractAddr, err := ioIDStoreAbi.Unpack("projectDeviceContract", data)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to unpack project device contract response", err)
	}

	data, err = ioIDStoreAbi.Pack("projectAppliedAmount", new(big.Int).SetUint64(projectId))
	if err != nil {
		return output.NewError(output.ConvertError, "failed to pack project applied amount arguments", err)
	}
	res, err = action.Read(ioioIDStore, "0", data)
	if err != nil {
		return output.NewError(output.APIError, "failed to read contract", err)
	}
	data, _ = hex.DecodeString(res)
	projectAppliedAmount, err := ioIDStoreAbi.Unpack("projectAppliedAmount", data)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to unpack project applied amount response", err)
	}

	data, err = ioIDStoreAbi.Pack("projectActivedAmount", new(big.Int).SetUint64(projectId))
	if err != nil {
		return output.NewError(output.ConvertError, "failed to pack project actived amount arguments", err)
	}
	res, err = action.Read(ioioIDStore, "0", data)
	if err != nil {
		return output.NewError(output.APIError, "failed to read contract", err)
	}
	data, _ = hex.DecodeString(res)
	projectActivedAmount, err := ioIDStoreAbi.Unpack("projectActivedAmount", data)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to unpack project actived amount response", err)
	}

	fmt.Printf(`Project #%d detail:
{
	"projectContractAddress": "%s",
	"deviceNFT": "%s",
	"appliedIoIDs": "%s",
	"activedIoIDs": "%s",
}
`,
		projectId,
		projectAddr[0].(address.Address).String(),
		deviceContractAddr[0].(address.Address).String(),
		projectAppliedAmount[0].(*big.Int).String(),
		projectActivedAmount[0].(*big.Int).String(),
	)

	return nil
}
