package action

import (
	"encoding/hex"
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/output"
)

// stakeRenewCmd represents the stake renew command
var stakeRenewCmd = &cobra.Command{
	Use: "renew BUCKET_INDEX STAKE_DURATION [DATA] [--auto-restake] [-c ALIAS|CONTRACT_ADDRESS]" +
		" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GASPRICE] [-P PASSWORD] [-y]",
	Short: "Renew bucket on IoTeX blockchain",
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := renew(args)
		return output.PrintError(err)
	},
}

func init() {
	registerWriteCommand(stakeRenewCmd)
	stakeRenewCmd.Flags().BoolVar(&autoRestake, "auto-restake", false, "auto restake without power decay")
}

func renew(args []string) error {
	bucketIndex, ok := new(big.Int).SetString(args[0], 10)
	if !ok {
		return output.NewError(output.ConvertError, "failed to convert bucket index", nil)
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

	bytecode, err := stakeABI.Pack("restake", bucketIndex, stakeDuration, autoRestake, data)
	if err != nil {
		return output.NewError(output.ConvertError, "cannot generate bytecode from given command", err)
	}

	return Execute(contract.String(), big.NewInt(0), bytecode)
}
