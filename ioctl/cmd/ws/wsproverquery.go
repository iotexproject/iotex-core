package ws

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
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

var wsProverQueryVmTypeCmd = &cobra.Command{
	Use: "queryvmtype",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "query prover vm type",
		config.Chinese: "查询prover节点虚拟机类型信息",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := queryProverVmType(big.NewInt(int64(proverID.Value().(uint64))), big.NewInt(int64(proverVmType.Value().(uint64))))
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

	proverID.RegisterCommand(wsProverQueryVmTypeCmd)
	proverID.MarkFlagRequired(wsProverQueryVmTypeCmd)
	proverVmType.RegisterCommand(wsProverQueryVmTypeCmd)
	proverVmType.MarkFlagRequired(wsProverQueryVmTypeCmd)

	wsProverCmd.AddCommand(wsProverQueryCmd)
	wsProverCmd.AddCommand(wsProverQueryVmTypeCmd)
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

func queryProverVmType(proverID, vmType *big.Int) (string, error) {
	caller, err := NewContractCaller(proverStoreABI, proverStoreAddress)
	if err != nil {
		return "", errors.Wrap(err, "failed to new contract caller")
	}
	result := NewContractResult(&proverStoreABI, funcQueryProverVmType, new(bool))
	if err = caller.Read(funcQueryProverVmType, []any{proverID, vmType}, result); err != nil {
		return "", errors.Wrapf(err, "failed to read contract: %s", funcQueryProverVmType)
	}

	hasVmType, err := result.Result()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("the state of proverID and vmType is %t", *hasVmType.(*bool)), nil
}
