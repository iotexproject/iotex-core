package ws

import (
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/ws/contracts"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
)

var wsProjectRegisterCmd = &cobra.Command{
	Use: "register",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "register w3bstream project",
		config.Chinese: "创建w3bstream项目",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := registerProject(int64(projectID.Value().(uint64)))
		if err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(output.JSONString(out))
		return nil
	},
}

func init() {
	projectID.RegisterCommand(wsProjectRegisterCmd)
	_ = wsProjectRegisterCmd.MarkFlagRequired(projectID.Label())

	transferAmount.RegisterCommand(wsProjectRegisterCmd)

	wsProject.AddCommand(wsProjectRegisterCmd)
}

func registerProject(projectID int64) (any, error) {
	caller, err := NewContractCaller(projectRegistrarABI, projectRegistrarAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create contract caller")
	}
	amount, ok := new(big.Int).SetString(transferAmount.Value().(string), 10)
	if !ok {
		return nil, errors.Errorf("invalid transfer amount flag: %v", transferAmount.Value())
	}
	caller.SetAmount(amount)

	result := NewContractResult(&projectStoreABI, eventOnProjectRegistered, new(contracts.W3bstreamProjectProjectBinded))

	_, err = caller.CallAndRetrieveResult(funcProjectRegister, []any{big.NewInt(projectID)}, result)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to call contract: %s.%s", projectRegistrarAddr, funcProjectRegister)
	}

	v, err := result.Result()
	if err != nil {
		return nil, err
	}

	return queryProject(v.(*contracts.W3bstreamProjectProjectBinded).ProjectId)
}
