package ws

import (
	"fmt"
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/ws/contracts"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

var wsProverAddNodeTypeCmd = &cobra.Command{
	Use: "addtype",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "add prover node type",
		config.Chinese: "添加prover节点类型",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := addProverType(
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

var wsProverDelNodeTypeCmd = &cobra.Command{
	Use: "deltype",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "delete prover node type",
		config.Chinese: "删除prover节点类型",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := delProverType(
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
	proverID.RegisterCommand(wsProverAddNodeTypeCmd)
	proverID.MarkFlagRequired(wsProverAddNodeTypeCmd)

	proverNodeType.RegisterCommand(wsProverAddNodeTypeCmd)
	proverNodeType.MarkFlagRequired(wsProverAddNodeTypeCmd)

	proverID.RegisterCommand(wsProverDelNodeTypeCmd)
	proverID.MarkFlagRequired(wsProverDelNodeTypeCmd)

	proverNodeType.RegisterCommand(wsProverDelNodeTypeCmd)
	proverNodeType.MarkFlagRequired(wsProverDelNodeTypeCmd)

	wsProverCmd.AddCommand(wsProverAddNodeTypeCmd)
	wsProverCmd.AddCommand(wsProverDelNodeTypeCmd)
}

func addProverType(proverID, nodeType *big.Int) (string, error) {
	caller, err := NewContractCaller(proverStoreABI, proverStoreAddress)
	if err != nil {
		return "", errors.Wrap(err, "failed to create contract caller")
	}

	value := new(contracts.W3bstreamProverNodeTypeAdded)
	result := NewContractResult(&proverStoreABI, eventOnProverTypeAdded, value)
	if _, err = caller.CallAndRetrieveResult(funcAddProverType, []any{proverID, nodeType}, result); err != nil {
		return "", errors.Wrap(err, "failed to call contract")
	}

	if _, err = result.Result(); err != nil {
		return "", err
	}

	return fmt.Sprintf("the type of proverID %s and nodeType %s has added successfully.", value.Id.String(), value.Typ.String()), nil
}

func delProverType(proverID, nodeType *big.Int) (string, error) {
	caller, err := NewContractCaller(proverStoreABI, proverStoreAddress)
	if err != nil {
		return "", errors.Wrap(err, "failed to create contract caller")
	}

	value := new(contracts.W3bstreamProverNodeTypeDeleted)
	result := NewContractResult(&proverStoreABI, eventOnProverTypeDeleted, value)
	if _, err = caller.CallAndRetrieveResult(funcDelProverType, []any{proverID, nodeType}, result); err != nil {
		return "", errors.Wrap(err, "failed to call contract")
	}

	if _, err = result.Result(); err != nil {
		return "", err
	}

	return fmt.Sprintf("the type of proverID %s and nodeType %s has deleted successfully.", value.Id.String(), value.Typ.String()), nil
}
