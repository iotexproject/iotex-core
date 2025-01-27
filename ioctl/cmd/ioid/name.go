package ioid

import (
	"fmt"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/ws"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
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
		config.ReadConfig.IoidProjectStoreContract,
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

	projectAddr, err := readContract(ioioIDStore, ioIDStoreABI, "project", "0")
	if err != nil {
		return output.NewError(output.ConvertError, "failed to read project", err)
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
