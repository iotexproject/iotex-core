package ioid

import (
	"bytes"
	_ "embed" // used to embed contract abi
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-address/address"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
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
		config.ReadConfig.IoidProjectStoreContract,
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

	projectAddr, err := readContract(ioioIDStore, ioIDStoreABI, "project", "0")
	if err != nil {
		return output.NewError(output.ConvertError, "failed to read project", err)
	}

	ioProjectAddr, _ := address.FromHex(projectAddr[0].(address.Address).String())
	name, err := readContract(ioProjectAddr, projectABI, "name", "0", new(big.Int).SetUint64(projectId))
	if err != nil {
		return output.NewError(output.ConvertError, "failed to read project name", err)
	}
	owner, err := readContract(ioProjectAddr, projectABI, "ownerOf", "0", new(big.Int).SetUint64(projectId))
	if err != nil {
		return output.NewError(output.ConvertError, "failed to read project owner", err)
	}

	deviceContractAddr, err := readContract(ioioIDStore, ioIDStoreABI, "projectDeviceContract", "0", new(big.Int).SetUint64(projectId))
	if err != nil {
		return output.NewError(output.ConvertError, "failed to read project device contract", err)
	}
	projectAppliedAmount, err := readContract(ioioIDStore, ioIDStoreABI, "projectAppliedAmount", "0", new(big.Int).SetUint64(projectId))
	if err != nil {
		return output.NewError(output.ConvertError, "failed to read project device applied amount", err)
	}
	projectActivedAmount, err := readContract(ioioIDStore, ioIDStoreABI, "projectActivedAmount", "0", new(big.Int).SetUint64(projectId))
	if err != nil {
		return output.NewError(output.ConvertError, "failed to read project actived amount", err)
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
