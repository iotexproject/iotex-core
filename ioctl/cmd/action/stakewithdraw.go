package action

import (
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
	return bucketAction("withdraw", args)
}
