package ws

import (
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

var wsProverQueryCmd = &cobra.Command{
	Use: "query",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "query prover",
		config.Chinese: "查询prover节点信息",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := queryProver(big.NewInt(int64(proverID.Value().(uint64))))
		if err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(output.JSONString(out))
		return nil
	},
}

func init() {
	proverID.RegisterCommand(wsProverQueryCmd)
	proverID.MarkFlagRequired(wsProverQueryCmd)

	wsProverCmd.AddCommand(wsProverQueryCmd)
}

func queryProver(proverID *big.Int) (any, error) {
	caller, err := NewContractCaller(proverStoreABI, proverStoreAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to new contract caller")
	}
	nodeType := new(big.Int)
	result := NewContractResult(&proverStoreABI, funcQueryProverNodeType, nodeType)
	if err = caller.Read(funcQueryProverNodeType, []any{proverID}, result); err != nil {
		return nil, errors.Wrapf(err, "failed to read contract: %s", funcQueryProverNodeType)
	}
	if _, err = result.Result(); err != nil {
		return nil, err
	}

	isPaused := new(bool)
	result2 := NewContractResult(&proverStoreABI, funcQueryProverIsPaused, isPaused)
	if err = caller.Read(funcQueryProverIsPaused, []any{proverID}, result2); err != nil {
		return nil, errors.Wrapf(err, "failed to read contract: %s", funcQueryProverIsPaused)
	}
	if _, err = result2.Result(); err != nil {
		return nil, err
	}

	return &struct {
		ProverID uint64 `json:"proverID"`
		NodeType uint64 `json:"nodeType"`
		IsPaused bool   `json:"isPaused"`
	}{
		ProverID: proverID.Uint64(),
		NodeType: nodeType.Uint64(),
		IsPaused: *isPaused,
	}, nil

}
