package ws

import (
	"fmt"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	// wsProjectQuery represents the query w3bstream project command
	wsProjectQuery = &cobra.Command{
		Use:   "query",
		Short: config.TranslateInLang(wsProjectQueryShorts, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := cmd.Flags().GetUint64("project-id")
			if err != nil {
				return output.PrintError(err)
			}
			out, err := queryProject(id)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(out)
			return nil
		},
	}

	// wsProjectQueryShorts query w3bstream project shorts multi-lang support
	wsProjectQueryShorts = map[config.Language]string{
		config.English: "query w3bstream project",
		config.Chinese: "查询项目",
	}
)

func init() {
	wsProjectQuery.Flags().Uint64P("project-id", "i", 0, config.TranslateInLang(_flagProjectIDUsages, config.UILanguage))

	_ = wsProjectQuery.MarkFlagRequired("project-id")
}

func queryProject(id uint64) (string, error) {
	contract, err := util.Address(wsProjectRegisterContractAddress)
	if err != nil {
		return "", output.NewError(output.AddressError, "failed to get project register contract address", err)
	}

	bytecode, err := wsProjectRegisterContractABI.Pack(queryWsProjectFuncName, id)
	if err != nil {
		return "", output.NewError(output.ConvertError, fmt.Sprintf("failed to pack abi"), err)
	}

	addr, _ := address.FromString(contract)
	data, err := action.Read(addr, "0", bytecode)
	if err != nil {
		return "", errors.Wrap(err, "read contract failed")
	}
	return parseOutput(&wsProjectRegisterContractABI, queryWsProjectFuncName, data)
}
