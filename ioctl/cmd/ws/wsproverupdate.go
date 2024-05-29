package ws

import (
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/ws/contracts"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

var wsProverUpdateCmd = &cobra.Command{
	Use: "update",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "update prover",
		config.Chinese: "更新prover节点类型",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := updateProver(
			big.NewInt(int64(proverID.Value().(uint64))),
			big.NewInt(int64(proverNodeType.Value().(uint64))),
		)
		if err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(output.JSONString(out))
		return nil
	},
}

func init() {
	proverID.RegisterCommand(wsProverUpdateCmd)
	proverID.MarkFlagRequired(wsProverUpdateCmd)

	proverNodeType.RegisterCommand(wsProverUpdateCmd)
	proverNodeType.MarkFlagRequired(wsProverUpdateCmd)

	wsProverCmd.AddCommand(wsProverUpdateCmd)
}

func updateProver(proverID, nodeType *big.Int) (any, error) {
	caller, err := NewContractCaller(proverStoreABI, proverStoreAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create contract caller")
	}

	value := new(contracts.W3bstreamProverNodeTypeUpdated)
	result := NewContractResult(&proverStoreABI, eventOnProverUpdated, value)
	if _, err = caller.CallAndRetrieveResult(funcUpdateProver, []any{proverID, nodeType}, result); err != nil {
		return nil, errors.Wrap(err, "failed to call contract")
	}

	if _, err = result.Result(); err != nil {
		return nil, err
	}

	return &struct {
		ProverID *big.Int `json:"proverID"`
		NodeType *big.Int `json:"nodeType"`
	}{
		ProverID: value.Id,
		NodeType: value.Typ,
	}, nil
}
