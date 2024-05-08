package ws

import (
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/ws/contracts"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

var wsProjectRegisterCmd = &cobra.Command{
	Use: "register",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "register w3bstream project",
		config.Chinese: "创建w3bstream项目",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := registerProject()
		if err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(output.JSONString(out))
		return nil
	},
}

func init() {
	wsProject.AddCommand(wsProjectRegisterCmd)
}

func registerProject() (any, error) {
	caller, err := NewContractCaller(projectRegistrarABI, projectRegistrarAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create contract caller")
	}

	result := NewContractResult(&projectStoreABI, eventOnProjectRegistered, new(contracts.W3bstreamProjectTransfer))

	_, err = caller.CallAndRetrieveResult(funcProjectRegister, nil, result)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to call contract: %s.%s", projectRegistrarAddr, funcProjectRegister)
	}

	v, err := result.Result()
	if err != nil {
		return nil, err
	}
	return &struct {
		ProjectId *big.Int `json:"projectId"`
		Owner     string   `json:"owner"`
	}{
		ProjectId: v.(*contracts.W3bstreamProjectTransfer).TokenId,
		Owner:     caller.Sender().String(),
	}, nil
}
