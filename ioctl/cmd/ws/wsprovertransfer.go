package ws

import (
	"encoding/hex"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/ws/contracts"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

var wsProverTransferCmd = &cobra.Command{
	Use: "transfer",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "transfer prover operator",
		config.Chinese: "更换prover操作者",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		operator := proverOperator.Value().(string)
		addr, err := address.FromString(operator)
		if err != nil {
			return output.PrintError(errors.Wrapf(err, "invalid operator address: %s", operator))
		}
		id := big.NewInt(int64(proverID.Value().(uint64)))
		newoperator := common.BytesToAddress(addr.Bytes())

		output.PrintResult(hex.EncodeToString(newoperator[:]))
		output.PrintResult(addr.String())

		out, err := transfer(id, newoperator)
		if err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(output.JSONString(out))
		return nil
	},
}

func init() {
	proverID.RegisterCommand(wsProverTransferCmd)
	proverID.MarkFlagRequired(wsProverTransferCmd)

	proverOperator.RegisterCommand(wsProverTransferCmd)
	proverOperator.MarkFlagRequired(wsProverTransferCmd)

	wsProverCmd.AddCommand(wsProverTransferCmd)
}

func transfer(proverID *big.Int, operator common.Address) (any, error) {
	caller, err := NewContractCaller(proverStoreABI, proverStoreAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create contract caller")
	}

	value := new(contracts.W3bstreamProverOperatorSet)
	result := NewContractResult(&proverStoreABI, eventOnProverOwnerChanged, value)
	if _, err = caller.CallAndRetrieveResult(funcChangeProverOwner, []any{proverID, operator}, result); err != nil {
		return nil, errors.Wrap(err, "failed to call contract")
	}

	if _, err = result.Result(); err != nil {
		return nil, err
	}

	newoperator, err := address.FromBytes(value.Operator[:])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to convert address: %s", hex.EncodeToString(value.Operator[:]))
	}

	return &struct {
		ProverID         *big.Int `json:"proverID"`
		PreviousOperator string   `json:"previousOperator"`
		NewOperator      string   `json:"newOperator"`
	}{
		ProverID:         value.Id,
		PreviousOperator: caller.Sender().String(),
		NewOperator:      newoperator.String(),
	}, nil
}
