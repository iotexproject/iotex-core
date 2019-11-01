// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
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

var autoRestake bool
var stakingContractAddress string
var stakeABI abi.ABI

//StakeCmd represent stake command
var StakeCmd = &cobra.Command{
	Use:   "stake",
	Short: "Supporting native staking from ioctl",
}

func init() {
	StakeCmd.AddCommand(stakeStakeCmd)
	StakeCmd.AddCommand(stakeRestakeCmd)
	StakeCmd.AddCommand(stakeStoreCmd)

	//TODO: initialize stakingContractAddress by setting default value for `--staking-contract-address` flag
	StakeCmd.PersistentFlags().StringVarP(&stakingContractAddress, "staking-contract-address", "c",
		"", "set staking contract address")
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
