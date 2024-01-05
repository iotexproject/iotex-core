package ws

import (
	"fmt"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

var (
	wsProjectAddOperatorShorts = map[config.Language]string{
		config.English: "add operator to project",
		config.Chinese: "添加项目操作者",
	}

	wsProjectDelOperatorShorts = map[config.Language]string{
		config.English: "remove operator to project",
		config.Chinese: "移除项目操作者",
	}

	wsProjectAddOperator = &cobra.Command{
		Use:   "addoperator",
		Short: config.TranslateInLang(wsProjectAddOperatorShorts, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := cmd.Flags().GetUint64("project-id")
			if err != nil {
				return output.PrintError(err)
			}
			op, err := cmd.Flags().GetString("operator-address")
			if err != nil || op == "" {
				return output.PrintError(errors.New("invalid operator"))
			}
			out, err := operator(id, op, opOperatorAdd)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(out)
			return nil
		},
	}

	wsProjectDelOperator = &cobra.Command{
		Use:   "deloperator",
		Short: config.TranslateInLang(wsProjectDelOperatorShorts, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := cmd.Flags().GetUint64("project-id")
			if err != nil {
				return output.PrintError(err)
			}
			op, err := cmd.Flags().GetString("operator-address")
			if err != nil {
				return output.PrintError(err)
			}
			out, err := operator(id, op, opOperatorDel)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(out)
			return nil
		},
	}
)

const (
	opOperatorAdd = 0
	opOperatorDel = 1
)

func init() {
	wsProjectAddOperator.Flags().Uint64P("project-id", "i", 0, config.TranslateInLang(_flagProjectIDUsages, config.UILanguage))
	wsProjectAddOperator.Flags().StringP("operator-address", "a", "", config.TranslateInLang(_flagProjectOperatorUsages, config.UILanguage))
	wsProjectDelOperator.Flags().Uint64P("project-id", "i", 0, config.TranslateInLang(_flagProjectIDUsages, config.UILanguage))
	wsProjectDelOperator.Flags().StringP("operator-address", "a", "", config.TranslateInLang(_flagProjectOperatorUsages, config.UILanguage))

	_ = wsProjectAddOperator.MarkFlagRequired("project-id")
	_ = wsProjectAddOperator.MarkFlagRequired("operator-address")
	_ = wsProjectDelOperator.MarkFlagRequired("project-id")
	_ = wsProjectDelOperator.MarkFlagRequired("operator-address")
}

func operator(projectID uint64, operator string, op int) (string, error) {
	var funcName string
	switch op {
	case opOperatorAdd:
		funcName = addProjectOperatorFuncName
	case opOperatorDel:
		funcName = delProjectOperatorFuncName
	default:
		return "", errors.New("invalid operate")
	}

	operatorAddr, err := address.FromString(operator)
	if err != nil {
		return "", output.NewError(output.AddressError, "invalid operator address", err)
	}

	contract, err := util.Address(wsProjectRegisterContractAddress)
	if err != nil {
		return "", output.NewError(output.AddressError, "failed to get project register contract address", err)
	}

	bytecode, err := wsProjectRegisterContractABI.Pack(funcName, projectID, operatorAddr)
	if err != nil {
		return "", output.NewError(output.ConvertError, fmt.Sprintf("failed to pack abi"), err)
	}

	if err = action.Execute(contract, big.NewInt(0), bytecode); err != nil {
		return "", errors.Wrap(err, "failed to execute contract")
	}

	if op == opOperatorAdd {
		return fmt.Sprintf("operatro %s added", operator), nil
	}
	return fmt.Sprintf("operatro %s removed", operator), nil
}
