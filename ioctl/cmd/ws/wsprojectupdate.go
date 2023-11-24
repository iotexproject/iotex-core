package ws

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
	// wsProjectUpdate represents the update w3bstream project command
	wsProjectUpdate = &cobra.Command{
		Use:   "update",
		Short: config.TranslateInLang(wsProjectUpdateShorts, config.UILanguage),
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

	// wsProjectUpdateShorts update w3bstream project shorts multi-lang support
	wsProjectUpdateShorts = map[config.Language]string{
		config.English: "update w3bstream project",
		config.Chinese: "更新项目",
	}
)

func init() {
	wsProjectUpdate.Flags().Uint64P("project-id", "p", 0, config.TranslateInLang(_flagProjectIDUsages, config.UILanguage))
	wsProjectUpdate.Flags().StringP("project-uri", "u", "", config.TranslateInLang(_flagProjectUriUsages, config.UILanguage))
	wsProjectUpdate.Flags().StringP("project-hash", "h", "", config.TranslateInLang(_flagProjectHashUsages, config.UILanguage))

	_ = wsProjectCreate.MarkFlagRequired("project-id")
	_ = wsProjectCreate.MarkFlagRequired("project-uri")
	_ = wsProjectCreate.MarkFlagRequired("project-hash")
}

func updateProject(projectID uint64, uri, hash string) error {
	contract, err := util.Address(wsProjectRegisterContractAddress)
	if err != nil {
		return output.NewError(output.AddressError, "failed to get project register contract address", err)
	}

	hashArg, err := convertStringToAbiBytes32(hash)
	if err != nil {
		return errors.Wrap(err, "convert input arg failed")
	}

	bytecode, err := wsProjectRegisterContractABI.Pack(
		updateWsProjectFuncName,
		projectID, uri, hashArg,
	)
	if err != nil {
		return output.NewError(output.ConvertError, fmt.Sprintf("failed to pack abi"), err)
	}

	return action.Execute(contract, big.NewInt(0), bytecode)
}
