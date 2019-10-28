// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/log"
)

var autoStake bool
var stackingContractAddress string

// actionStakeCmd represents the action stake command
var actionStakeCmd = &cobra.Command{
	Use: "stake AMOUNT_IOTX CANDIDATE_NAME STAKE_DURATION [DATA] [--auto-stake] [-c ALIAS|CONTRACT_ADDRESS]" +
		" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GASPRICE] [-P PASSWORD] [-y]",
	Short: "Native staking on IoTeX blockchain",
	Args:  cobra.RangeArgs(3, 4),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := stake(args)
		return output.PrintError(err)
	},
}

// actionRestakeCmd represents the action stake command
var actionRestakeCmd = &cobra.Command{
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
	//TODO: initialize stackingContractAddress by setting default value for `--staking-contract-address` flag
	actionStakeCmd.Flags().StringVarP(&stackingContractAddress, "staking-contract-address", "c",
		"", "set staking contract address")
	actionRestakeCmd.Flags().StringVarP(&stackingContractAddress, "staking-contract-address", "c",
		"", "set staking contract address")

	actionStakeCmd.Flags().BoolVar(&autoStake, "auto-stake", false, "auto stake without power decay")
	actionRestakeCmd.Flags().BoolVar(&autoStake, "auto-stake", false, "auto stake without power decay")

	registerWriteCommand(actionStakeCmd)
	registerWriteCommand(actionRestakeCmd)
}

func stackingContract() (address.Address, error) {
	return alias.IOAddress(stackingContractAddress)
}

func stake(args []string) error {
	amount, err := util.StringToRau(args[0], util.IotxDecimalNum)
	if err != nil {
		return output.NewError(output.ConvertError, "invalid IOTX amount", err)
	}
	candidateName := args[1]
	stakeDuration, ok := new(big.Int).SetString(args[2], 10)
	if !ok {
		return output.NewError(output.ConvertError, "failed to convert stake duration", nil)
	}
	// TODO: check whether stake duration is in valid range
	var data []byte
	if len(args) == 4 {
		data = make([]byte, 2*len([]byte(args[3])))
		hex.Encode(data, []byte(args[3]))
	}
	contract, err := stackingContract()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get contract address", err)
	}
	stakeABI, err := abi.JSON(strings.NewReader(poll.NsAbi))
	if err != nil {
		log.L().Panic("cannot get abi JSON data", zap.Error(err))
	}
	bytecode, err := stakeABI.Pack("createPygg", candidateName, stakeDuration, autoStake, data)
	if err != nil {
		return output.NewError(output.ConvertError, "cannot generate bytecode from given command", err)
	}
	return Execute(contract.String(), amount, bytecode)
}

func restake(args []string) error {
	pyggIndex, ok := new(big.Int).SetString(args[0], 10)
	if !ok {
		return output.NewError(output.ConvertError, "failed to convert pygg index", nil)
	}
	stakeDuration, ok := new(big.Int).SetString(args[1], 10)
	if !ok {
		return output.NewError(output.ConvertError, "failed to convert stake duration", nil)
	}
	// TODO: check whether stake duration is in valid range
	var data []byte
	if len(args) == 3 {
		data = make([]byte, 2*len([]byte(args[2])))
		hex.Encode(data, []byte(args[2]))
	}
	contract, err := stackingContract()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get contract address", err)
	}
	stakeABI, err := abi.JSON(strings.NewReader(poll.NsAbi))
	if err != nil {
		log.L().Panic("cannot get abi JSON data", zap.Error(err))
	}
	bytecode, err := stakeABI.Pack("restake", pyggIndex, stakeDuration, autoStake, data)
	if err != nil {
		return output.NewError(output.ConvertError, "cannot generate bytecode from given command", err)
	}
	return Execute(contract.String(), big.NewInt(0), bytecode)
}
