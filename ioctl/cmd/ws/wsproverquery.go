package ws

import (
	"fmt"
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

var wsProverQueryTypeCmd = &cobra.Command{
	Use: "querytype",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "query prover node type",
		config.Chinese: "查询prover节点类型信息",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := queryProverType(big.NewInt(int64(proverID.Value().(uint64))), big.NewInt(int64(proverNodeType.Value().(uint64))))
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

	proverID.RegisterCommand(wsProverQueryTypeCmd)
	proverID.MarkFlagRequired(wsProverQueryTypeCmd)
	proverNodeType.RegisterCommand(wsProverQueryTypeCmd)
	proverNodeType.MarkFlagRequired(wsProverQueryTypeCmd)

	wsProverCmd.AddCommand(wsProverQueryCmd)
	wsProverCmd.AddCommand(wsProverQueryTypeCmd)
}

func queryProver(proverID *big.Int) (any, error) {
	caller, err := NewContractCaller(proverStoreABI, proverStoreAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to new contract caller")
	}

	result := NewContractResult(&proverStoreABI, funcQueryProverIsPaused, new(bool))
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
		IsPaused bool   `json:"isPaused"`
		Operator string `json:"operator"`
	}{
		ProverID: proverID.Uint64(),
		Prover:   ownerAddr,
		IsPaused: *isPaused.(*bool),
		Operator: operatorAddr,
	}, nil

}

func queryProverType(proverID, nodeType *big.Int) (string, error) {
	caller, err := NewContractCaller(proverStoreABI, proverStoreAddress)
	if err != nil {
		return "", errors.Wrap(err, "failed to new contract caller")
	}
	result := NewContractResult(&proverStoreABI, funcQueryProverNodeType, new(bool))
	if err = caller.Read(funcQueryProverNodeType, []any{proverID, nodeType}, result); err != nil {
		return "", errors.Wrapf(err, "failed to read contract: %s", funcQueryProverNodeType)
	}

	hasNodeType, err := result.Result()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("the state of proverID and nodeType is %t", *hasNodeType.(*bool)), nil
}
