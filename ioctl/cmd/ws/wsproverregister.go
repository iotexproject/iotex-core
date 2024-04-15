package ws

import (
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/ws/contracts"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

var wsProverRegisterCmd = &cobra.Command{
	Use: "register",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "register prover",
		config.Chinese: "注册prover节点",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := registerProver()
		if err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(output.JSONString(out))
		return nil
	},
}

func init() {
	wsProverCmd.AddCommand(wsProverRegisterCmd)
}

func registerProver() (any, error) {
	caller, err := NewContractCaller(fleetManagementABI, fleetManagementAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create contract caller")
	}

	value := new(contracts.W3bstreamProverTransfer)
	result := NewContractResult(&proverStoreABI, eventOnProverRegistered, value)
	if _, err = caller.CallAndRetrieveResult(funcProverRegister, nil, result); err != nil {
		return nil, errors.Wrap(err, "failed to call contract")
	}

	if _, err = result.Result(); err != nil {
		return nil, err
	}

	return &struct {
		ProverID *big.Int `json:"proverID"`
		Owner    string
	}{
		ProverID: value.TokenId,
		Owner:    caller.Sender().String(),
	}, nil
}
