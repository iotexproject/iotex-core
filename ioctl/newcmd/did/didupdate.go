// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package did

import (
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/action"
)

// Multi-language support
var (
	_updateCmdUses = map[config.Language]string{
		config.English: "update (CONTRACT_ADDRESS|ALIAS) hash uri",
		config.Chinese: "update (合约地址|别名) hash uri",
	}
	_updateCmdShorts = map[config.Language]string{
		config.English: "Update DID on IoTeX blockchain",
		config.Chinese: "在IoTeX链上更新DID",
	}
)

// NewDidUpdateCmd represents the contract invoke update command
func NewDidUpdateCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_updateCmdUses)
	short, _ := client.SelectTranslation(_updateCmdShorts)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			contract, err := client.Address(args[0])
			if err != nil {
				return errors.Wrap(err, "failed to get contract address")
			}
			bytecode, err := encode(_updateDIDName, args[1], args[2])
			if err != nil {
				return errors.Wrap(err, "failed to decode data")
			}
			gasPrice, signer, password, nonce, gasLimit, assumeYes, err := action.GetWriteCommandFlag(cmd)
			if err != nil {
				return err
			}
			return action.Execute(client, cmd, contract, big.NewInt(0), bytecode, gasPrice, signer, password, nonce, gasLimit, assumeYes)
		},
	}
	action.RegisterWriteCommand(client, cmd)
	return cmd
}
