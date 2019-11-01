package action

import (
	"encoding/hex"
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/output"
)

// stakeRestakeCmd represents the stake stake command
var stakeRestakeCmd = &cobra.Command{
	// PYGG_INDEX is  BUCKET_INDEX
	Use: "restake PYGG_INDEX STAKE_DURATION [DATA] [--auto-stake] [-c ALIAS|CONTRACT_ADDRESS]" +
		" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GASPRICE] [-P PASSWORD] [-y]",
	Short: "Restake pygg on IoTeX blockchain",
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := restake(args)
		return output.PrintError(err)
	},
}

func init() {
	registerWriteCommand(stakeRestakeCmd)
	stakeRestakeCmd.Flags().BoolVar(&autoRestake, "auto-restake", false, "auto restake without power decay")
}

func restake(args []string) error {
	pyggIndex, ok := new(big.Int).SetString(args[0], 10)
	if !ok {
		return output.NewError(output.ConvertError, "failed to convert pygg index", nil)
	}

	stakeDuration, err := parseStakeDuration(args[1])
	if err != nil {
		return output.NewError(0, "", err)
	}

	data := []byte{}
	if len(args) == 3 {
		data = make([]byte, 2*len([]byte(args[2])))
		hex.Encode(data, []byte(args[2]))
	}

	contract, err := stakingContract()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get contract address", err)
	}

	bytecode, err := stakeABI.Pack("restake", pyggIndex, stakeDuration, autoRestake, data)
	if err != nil {
		return output.NewError(output.ConvertError, "cannot generate bytecode from given command", err)
	}

	return Execute(contract.String(), big.NewInt(0), bytecode)
}
