package ioid

import (
	"bytes"
	_ "embed" // used to embed contract abi
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-address/address"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
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

	//go:embed contracts/abis/Project.json
	projectJSON []byte
	projectABI  abi.ABI
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
	var err error
	projectABI, err = abi.JSON(bytes.NewReader(projectJSON))
	if err != nil {
		panic(err)
	}

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

	data, err := ioIDStoreABI.Pack("project")
	if err != nil {
		return output.NewError(output.ConvertError, "failed to pack project arguments", err)
	}
	res, err := action.Read(ioioIDStore, "0", data)
	if err != nil {
		return output.NewError(output.APIError, "failed to read contract", err)
	}
	data, _ = hex.DecodeString(res)
	projectAddr, err := ioIDStoreABI.Unpack("project", data)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to unpack project response", err)
	}

	ioProjectAddr, _ := address.FromHex(projectAddr[0].(address.Address).String())
	data, err = projectABI.Pack("name", new(big.Int).SetUint64(projectId))
	if err != nil {
		return output.NewError(output.ConvertError, "failed to pack project name arguments", err)
	}
	res, err = action.Read(ioProjectAddr, "0", data)
	if err != nil {
		return output.NewError(output.APIError, "failed to read contract", err)
	}
	data, _ = hex.DecodeString(res)
	name, err := projectABI.Unpack("name", data)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to unpack project name response", err)
	}
	data, err = projectABI.Pack("ownerOf", new(big.Int).SetUint64(projectId))
	if err != nil {
		return output.NewError(output.ConvertError, "failed to pack project ownerOf arguments", err)
	}
	res, err = action.Read(ioProjectAddr, "0", data)
	if err != nil {
		return output.NewError(output.APIError, "failed to read contract", err)
	}
	data, _ = hex.DecodeString(res)
	owner, err := projectABI.Unpack("ownerOf", data)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to unpack project ownerOf response", err)
	}

	data, err = ioIDStoreABI.Pack("projectDeviceContract", new(big.Int).SetUint64(projectId))
	if err != nil {
		return output.NewError(output.ConvertError, "failed to pack project device contract arguments", err)
	}
	res, err = action.Read(ioioIDStore, "0", data)
	if err != nil {
		return output.NewError(output.APIError, "failed to read contract", err)
	}
	data, _ = hex.DecodeString(res)
	deviceContractAddr, err := ioIDStoreABI.Unpack("projectDeviceContract", data)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to unpack project device contract response", err)
	}

	data, err = ioIDStoreABI.Pack("projectAppliedAmount", new(big.Int).SetUint64(projectId))
	if err != nil {
		return output.NewError(output.ConvertError, "failed to pack project applied amount arguments", err)
	}
	res, err = action.Read(ioioIDStore, "0", data)
	if err != nil {
		return output.NewError(output.APIError, "failed to read contract", err)
	}
	data, _ = hex.DecodeString(res)
	projectAppliedAmount, err := ioIDStoreABI.Unpack("projectAppliedAmount", data)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to unpack project applied amount response", err)
	}

	data, err = ioIDStoreABI.Pack("projectActivedAmount", new(big.Int).SetUint64(projectId))
	if err != nil {
		return output.NewError(output.ConvertError, "failed to pack project actived amount arguments", err)
	}
	res, err = action.Read(ioioIDStore, "0", data)
	if err != nil {
		return output.NewError(output.APIError, "failed to read contract", err)
	}
	data, _ = hex.DecodeString(res)
	projectActivedAmount, err := ioIDStoreABI.Unpack("projectActivedAmount", data)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to unpack project actived amount response", err)
	}

	fmt.Printf(`Project #%d detail:
{
	"projectContract": "%s",
	"name": "%s",
	"owner": "%s",
	"deviceNFT": "%s",
	"appliedIoIDs": "%s",
	"activedIoIDs": "%s",
}
`,
		projectId,
		projectAddr[0].(address.Address).String(),
		name[0].(string),
		owner[0].(address.Address).String(),
		deviceContractAddr[0].(address.Address).String(),
		projectAppliedAmount[0].(*big.Int).String(),
		projectActivedAmount[0].(*big.Int).String(),
	)

	return nil
}
