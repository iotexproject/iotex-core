package znode

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
	// znodeProjectQuery represents the query znode project command
	znodeProjectQuery = &cobra.Command{
		Use:   "query",
		Short: config.TranslateInLang(znodeProjectQueryShorts, config.UILanguage),
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

	// znodeProjectQueryShorts query znode project shorts multi-lang support
	znodeProjectQueryShorts = map[config.Language]string{
		config.English: "query znode project",
		config.Chinese: "查询项目",
	}
)

func init() {
	znodeProjectQuery.Flags().Uint64P("project-id", "i", 0, config.TranslateInLang(_flagProjectIDUsages, config.UILanguage))

	_ = znodeProjectQuery.MarkFlagRequired("project-id")
}

func queryProject(id uint64) (string, error) {
	contract, err := util.Address(znodeProjectRegisterContractAddress)
	if err != nil {
		return "", output.NewError(output.AddressError, "failed to get project register contract address", err)
	}

	bytecode, err := znodeProjectRegisterContractABI.Pack(queryZnodeProjectFuncName, id)
	if err != nil {
		return "", output.NewError(output.ConvertError, fmt.Sprintf("failed to pack abi"), err)
	}

	addr, _ := address.FromString(contract)
	data, err := action.Read(addr, "0", bytecode)
	if err != nil {
		return "", errors.Wrap(err, "read contract failed")
	}
	return parseOutput(&znodeProjectRegisterContractABI, queryZnodeProjectFuncName, data)
}
