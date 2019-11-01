package action

import (
	"encoding/hex"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/ioctl/validator"
)

// stakeStakeCmd represents the stake stake command
var stakeStakeCmd = &cobra.Command{
	Use: "stake AMOUNT_IOTX CANDIDATE_NAME STAKE_DURATION [DATA] [--auto-stake] [-c ALIAS|CONTRACT_ADDRESS]" +
		" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GASPRICE] [-P PASSWORD] [-y]",
	Short: "Create pygg on IoTeX blockchain",
	Args:  cobra.RangeArgs(3, 4),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := stake(args)
		return output.PrintError(err)
	},
}

func init() {
	registerWriteCommand(stakeStakeCmd)
	stakeStakeCmd.Flags().BoolVar(&autoRestake, "auto-restake", false, "auto restake without power decay")
}

func stake(args []string) error {
	amount, err := util.StringToRau(args[0], util.IotxDecimalNum)
	if err != nil {
		return output.NewError(output.ConvertError, "invalid IOTX amount", err)
	}

	if err := validator.ValidateCandidateName(args[1]); err != nil {
		return output.NewError(output.ValidationError, "invalid candidate name", err)
	}

	var candidateName [12]byte
	copy(candidateName[:], append(make([]byte, 12-len(args)), []byte(args[1])...))

	stakeDuration, err := parseStakeDuration(args[2])
	if err != nil {
		return output.NewError(0, "", err)
	}

	data := []byte{}
	if len(args) == 4 {
		data = make([]byte, 2*len([]byte(args[3])))
		hex.Encode(data, []byte(args[3]))
	}

	contract, err := stakingContract()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get contract address", err)
	}

	bytecode, err := stakeABI.Pack("createPygg", candidateName, stakeDuration, autoRestake, data)
	if err != nil {
		return output.NewError(output.ConvertError, "cannot generate bytecode from given command", err)
	}

	return Execute(contract.String(), amount, bytecode)
}
