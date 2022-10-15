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
	_deregisterCmdUses = map[config.Language]string{
		config.English: "deregister (CONTRACT_ADDRESS|ALIAS)",
		config.Chinese: "deregister (合约地址|别名)",
	}
	_deregisterCmdShorts = map[config.Language]string{
		config.English: "Deregister DID on IoTeX blockchain",
		config.Chinese: "Deregister 在IoTeX链上注销DID",
	}
)

// NewDeregisterCmd represents the contract invoke deregister command
func NewDeregisterCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_deregisterCmdUses)
	short, _ := client.SelectTranslation(_deregisterCmdShorts)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			contract, err := client.Address(args[0])
			if err != nil {
				return errors.Wrap(err, "failed to get contract address")
			}
			bytecode, err := _didABI.Pack(_deregisterDIDName)
			if err != nil {
				return errors.Wrap(err, "invalid bytecode")
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
