package ws

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
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
	result := NewContractResult(&proverStoreABI, funcQueryProverNodeType, new(big.Int))
	if err = caller.Read(funcQueryProverNodeType, []any{proverID}, result); err != nil {
		return nil, errors.Wrapf(err, "failed to read contract: %s", funcQueryProverNodeType)
	}

	nodeType, err := result.Result()
	if err != nil {
		return nil, err
	}

	result = NewContractResult(&proverStoreABI, funcQueryProverIsPaused, new(bool))
	if err = caller.Read(funcQueryProverIsPaused, []any{proverID}, result); err != nil {
		return nil, errors.Wrapf(err, "failed to read contract: %s", funcQueryProverIsPaused)
	}
	isPaused, err := result.Result()
	if err != nil {
		return nil, err
	}

	result = NewContractResult(&proverStoreABI, funcQueryProverOperator, &common.Address{})
	if err = caller.Read(funcQueryProverOperator, []any{proverID}, result); err != nil {
		return nil, errors.Wrapf(err, "failed to read contract: %s", funcQueryProverOperator)
	}
	operator, err := result.Result()
	if err != nil {
		return nil, err
	}

	result = NewContractResult(&proverStoreABI, funcQueryProverOwner, &common.Address{})
	if err = caller.Read(funcQueryProverOwner, []any{proverID}, result); err != nil {
		return nil, errors.Wrapf(err, "failed to read contract: %s", funcQueryProverOwner)
	}
	owner, err := result.Result()
	if err != nil {
		return nil, err
	}

	operatorAddr, err := util.Address((*operator.(*common.Address)).String())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to convert operator address")
	}

	ownerAddr, err := util.Address((*owner.(*common.Address)).String())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to convert prover address")
	}

	return &struct {
		ProverID uint64 `json:"proverID"`
		Prover   string `json:"owner"`
		NodeType uint64 `json:"nodeType"`
		IsPaused bool   `json:"isPaused"`
		Operator string `json:"operator"`
	}{
		ProverID: proverID.Uint64(),
		Prover:   ownerAddr,
		NodeType: nodeType.(*big.Int).Uint64(),
		IsPaused: *isPaused.(*bool),
		Operator: operatorAddr,
	}, nil

}
