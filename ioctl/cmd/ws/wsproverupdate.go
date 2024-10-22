package ws

import (
	"fmt"
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/ws/contracts"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
)

var wsProverAddVmTypeCmd = &cobra.Command{
	Use: "addvmtype",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "add prover vm type",
		config.Chinese: "添加prover节点虚拟机类型",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := addProverVmType(
			big.NewInt(int64(proverID.Value().(uint64))),
			big.NewInt(int64(proverVmType.Value().(uint64))),
		)
		if err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(output.JSONString(out))
		return nil
	},
}

var wsProverDelVmTypeCmd = &cobra.Command{
	Use: "delvmtype",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "delete prover vm type",
		config.Chinese: "删除prover节点虚拟机类型",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := delProverVmType(
			big.NewInt(int64(proverID.Value().(uint64))),
			big.NewInt(int64(proverVmType.Value().(uint64))),
		)
		if err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(output.JSONString(out))
		return nil
	},
}

func init() {
	proverID.RegisterCommand(wsProverAddVmTypeCmd)
	proverID.MarkFlagRequired(wsProverAddVmTypeCmd)

	proverVmType.RegisterCommand(wsProverAddVmTypeCmd)
	proverVmType.MarkFlagRequired(wsProverAddVmTypeCmd)

	proverID.RegisterCommand(wsProverDelVmTypeCmd)
	proverID.MarkFlagRequired(wsProverDelVmTypeCmd)

	proverVmType.RegisterCommand(wsProverDelVmTypeCmd)
	proverVmType.MarkFlagRequired(wsProverDelVmTypeCmd)

	wsProverCmd.AddCommand(wsProverAddVmTypeCmd)
	wsProverCmd.AddCommand(wsProverDelVmTypeCmd)
}

func addProverVmType(proverID, vmType *big.Int) (string, error) {
	caller, err := NewContractCaller(proverStoreABI, proverStoreAddress)
	if err != nil {
		return "", errors.Wrap(err, "failed to create contract caller")
	}

	value := new(contracts.W3bstreamProverVMTypeAdded)
	result := NewContractResult(&proverStoreABI, eventOnProverVmTypeAdded, value)
	if _, err = caller.CallAndRetrieveResult(funcAddProverVmType, []any{proverID, vmType}, result); err != nil {
		return "", errors.Wrap(err, "failed to call contract")
	}

	if _, err = result.Result(); err != nil {
		return "", err
	}

	return fmt.Sprintf("the type of proverID %s and vmType %s has added successfully.", value.Id.String(), value.Typ.String()), nil
}

func delProverVmType(proverID, vmType *big.Int) (string, error) {
	caller, err := NewContractCaller(proverStoreABI, proverStoreAddress)
	if err != nil {
		return "", errors.Wrap(err, "failed to create contract caller")
	}

	value := new(contracts.W3bstreamProverVMTypeDeleted)
	result := NewContractResult(&proverStoreABI, eventOnProverVmTypeDeleted, value)
	if _, err = caller.CallAndRetrieveResult(funcDelProverVmType, []any{proverID, vmType}, result); err != nil {
		return "", errors.Wrap(err, "failed to call contract")
	}

	if _, err = result.Result(); err != nil {
		return "", err
	}

	return fmt.Sprintf("the type of proverID %s and vmType %s has deleted successfully.", value.Id.String(), value.Typ.String()), nil
}
