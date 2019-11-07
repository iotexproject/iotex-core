package action

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/output"
)

// stakeReleaseCmd represents the stake release command
var stakeReleaseCmd = &cobra.Command{
	Use: "release BUCKET_INDEX [DATA] [-c ALIAS|CONTRACT_ADDRESS]" +
		" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GASPRICE] [-P PASSWORD] [-y]",
	Short: "Release bucket on IoTeX blockchain",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := release(args)
		return output.PrintError(err)
	},
}

func init() {
	registerWriteCommand(stakeReleaseCmd)
}

func release(args []string) error {
	return bucketAction("unstake", args)
}
