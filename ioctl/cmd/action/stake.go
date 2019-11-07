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
	"github.com/iotexproject/iotex-core/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/validator"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// MainnetStakingAddress stores native staking address as string
const MainnetStakingAddress = "io1xpq62aw85uqzrccg9y5hnryv8ld2nkpycc3gza"

var autoRestake bool
var stakingContractAddress string
var stakeABI abi.ABI

//StakeCmd represent stake command
var StakeCmd = &cobra.Command{
	Use:   "stake",
	Short: "Support native staking from ioctl",
}

func init() {
	StakeCmd.AddCommand(stakeCreateCmd)
	StakeCmd.AddCommand(stakeAddCmd)
	StakeCmd.AddCommand(stakeRenewCmd)
	StakeCmd.AddCommand(stakeReleaseCmd)
	StakeCmd.AddCommand(stakeWithdrawCmd)

	StakeCmd.PersistentFlags().StringVarP(&stakingContractAddress, "staking-contract-address", "c",
		MainnetStakingAddress, "set staking contract address")
	StakeCmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, "set endpoint for once")
	StakeCmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure,
		"insecure connection for once (default false)")

	var err error

	stakeABI, err = abi.JSON(strings.NewReader(poll.NsAbi))
	if err != nil {
		log.L().Panic("cannot get abi JSON data", zap.Error(err))
	}
}

func stakingContract() (address.Address, error) {
	addr, err := alias.IOAddress(stakingContractAddress)
	if err != nil {
		return nil, output.NewError(output.FlagError, "invalid staking contract address flag", err)
	}

	return addr, nil
}

func parseStakeDuration(stakeDurationString string) (*big.Int, error) {
	stakeDuration, ok := new(big.Int).SetString(stakeDurationString, 10)
	if !ok {
		return nil, output.NewError(output.ConvertError, "failed to convert stake duration", nil)
	}

	if err := validator.ValidateStakeDuration(stakeDuration); err != nil {
		return nil, output.NewError(output.ValidationError, "invalid stake duration", err)
	}

	return stakeDuration, nil
}

func bucketAction(function string, args []string) error {
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

	bytecode, err := stakeABI.Pack(function, bucketIndex, data)
	if err != nil {
		return output.NewError(output.ConvertError, "cannot generate bytecode from given command", err)
	}

	return Execute(contract.String(), big.NewInt(0), bytecode)
}
