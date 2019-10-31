// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/iotexproject/iotex-core/ioctl/cmd/config"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

var autoRestake bool
var stakingContractAddress string

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
}

func stakingContract() (address.Address, error) {
	addr, err := alias.IOAddress(stakingContractAddress)
	if err != nil {
		return nil, output.NewError(output.FlagError, "invalid staking contract address flag", err)
	}
	return addr, nil
}
