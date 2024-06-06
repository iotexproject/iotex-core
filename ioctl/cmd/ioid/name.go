package ioid

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/cmd/ws"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	_nameUsages = map[config.Language]string{
		config.English: "name [PROJECT_NAME]",
		config.Chinese: "name [项目名称]",
	}
	_nameShorts = map[config.Language]string{
		config.English: "Set project name",
		config.Chinese: "设置项目名称",
	}
)

// _nameCmd represents the ioID project name command
var _nameCmd = &cobra.Command{
	Use:   config.TranslateInLang(_nameUsages, config.UILanguage),
	Short: config.TranslateInLang(_nameShorts, config.UILanguage),
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		err := setName(args)
		return output.PrintError(err)
	},
}

func init() {
	_nameCmd.Flags().StringVarP(
		&ioIDStore, "ioIDStore", "i",
		"0xA0C9f9A884cdAE649a42F16b057735Bc4fE786CD",
		config.TranslateInLang(_ioIDStoreUsages, config.UILanguage),
	)
	_nameCmd.Flags().Uint64VarP(
		&projectId, "projectId", "p", 0,
		config.TranslateInLang(_projectIdUsages, config.UILanguage),
	)
}

func setName(args []string) error {
	name := args[0]

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

	caller, err := ws.NewContractCaller(projectABI, projectAddr[0].(address.Address).String())
	if err != nil {
		return output.NewError(output.SerializationError, "failed to create contract caller", err)
	}

	tx, err := caller.CallAndRetrieveResult("setName", []any{
		new(big.Int).SetUint64(projectId),
		name,
	})
	if err != nil {
		return output.NewError(output.SerializationError, "failed to call contract", err)
	}

	fmt.Printf("Set project name txHash: %s\n", tx)

	return nil
}
