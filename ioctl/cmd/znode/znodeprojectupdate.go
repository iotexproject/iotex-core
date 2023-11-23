package znode

import (
	"fmt"
	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"math/big"
)

var (
	// znodeProjectUpdate represents the update znode project command
	znodeProjectUpdate = &cobra.Command{
		Use:   "update",
		Short: config.TranslateInLang(znodeProjectUpdateShorts, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := cmd.Flags().GetUint64("project-id")
			if err != nil {
				return output.PrintError(err)
			}
			uri, err := cmd.Flags().GetString("project-uri")
			if err != nil {
				return output.PrintError(err)
			}
			hash, err := cmd.Flags().GetString("project-hash")
			if err != nil {
				return output.PrintError(err)
			}
			return output.PrintError(updateProject(id, uri, hash))
		},
	}

	// znodeProjectUpdateShorts update znode project shorts multi-lang support
	znodeProjectUpdateShorts = map[config.Language]string{
		config.English: "update znode project",
		config.Chinese: "更新项目",
	}
)

func init() {
	znodeProjectUpdate.Flags().Uint64P("project-id", "p", 0, config.TranslateInLang(_flagProjectIDUsages, config.UILanguage))
	znodeProjectUpdate.Flags().StringP("project-uri", "u", "", config.TranslateInLang(_flagProjectUriUsages, config.UILanguage))
	znodeProjectUpdate.Flags().StringP("project-hash", "h", "", config.TranslateInLang(_flagProjectHashUsages, config.UILanguage))

	_ = znodeProjectCreate.MarkFlagRequired("project-id")
	_ = znodeProjectCreate.MarkFlagRequired("project-uri")
	_ = znodeProjectCreate.MarkFlagRequired("project-hash")
}

func updateProject(projectID uint64, uri, hash string) error {
	contract, err := util.Address(znodeProjectRegisterContractAddress)
	if err != nil {
		return output.NewError(output.AddressError, "failed to get project register contract address", err)
	}

	hashArg, err := convertStringToAbiBytes32(hash)
	if err != nil {
		return errors.Wrap(err, "convert input arg failed")
	}

	bytecode, err := znodeProjectRegisterContractABI.Pack(
		updateZnodeProjectFuncName,
		projectID, uri, hashArg,
	)
	if err != nil {
		return output.NewError(output.ConvertError, fmt.Sprintf("failed to pack abi"), err)
	}

	return action.Execute(contract, big.NewInt(0), bytecode)
}
