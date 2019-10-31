package action

import (
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/ioctl/validator"
	"github.com/iotexproject/iotex-core/pkg/log"
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

	var candidateName [12]byte
	if err := validator.ValidateCandidateName(args[1]); err != nil {
		return output.NewError(output.ValidationError, "invalid candidate name", err)
	}
	copy(candidateName[:], append(make([]byte, 12-len(args)), []byte(args[1])...))

	stakeDuration, ok := new(big.Int).SetString(args[2], 10)
	if !ok {
		return output.NewError(output.ConvertError, "failed to convert stake duration", nil)
	}

	if err := validator.ValidateStakeDuration(stakeDuration); err != nil {
		return output.NewError(output.ValidationError, "invalid stake duration", err)
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

	stakeABI, err := abi.JSON(strings.NewReader(poll.NsAbi))
	if err != nil {
		log.L().Panic("cannot get abi JSON data", zap.Error(err))
	}

	bytecode, err := stakeABI.Pack("createPygg", candidateName, stakeDuration, autoRestake, data)
	if err != nil {
		return output.NewError(output.ConvertError, "cannot generate bytecode from given command", err)
	}

	return Execute(contract.String(), amount, bytecode)
}
