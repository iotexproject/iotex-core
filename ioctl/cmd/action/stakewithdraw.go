package action

import (
	"encoding/hex"
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/output"
)

// stakeWithdrawCmd represents the stake withdraw command
var stakeWithdrawCmd = &cobra.Command{
	Use: "withdraw BUCKET_INDEX [DATA] [-c ALIAS|CONTRACT_ADDRESS]" +
		" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GASPRICE] [-P PASSWORD] [-y]",
	Short: "Withdraw form bucket on IoTeX blockchain",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := withdraw(args)
		return output.PrintError(err)
	},
}

func init() {
	registerWriteCommand(stakeWithdrawCmd)
}

func withdraw(args []string) error {
	bucketIndex, ok := new(big.Int).SetString(args[0], 10)
	if !ok {
		return output.NewError(output.ConvertError, "failed to convert bucket index", nil)
	}

	data := []byte{}
	if len(args) == 2 {
		data = make([]byte, 2*len([]byte(args[1])))
		hex.Encode(data, []byte(args[1]))
	}

	contract, err := stakingContract()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get contract address", err)
	}

	bytecode, err := stakeABI.Pack("withdraw", bucketIndex, data)
	if err != nil {
		return output.NewError(output.ConvertError, "cannot generate bytecode from given command", err)
	}

	return Execute(contract.String(), big.NewInt(0), bytecode)
}
