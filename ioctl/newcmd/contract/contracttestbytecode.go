// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contract

import (
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/newcmd/action"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

// Multi-language support
var (
	_testBytecodeCmdUses = map[config.Language]string{
		config.English: "bytecode (CONTRACT_ADDRESS|ALIAS) PACKED_ARGUMENTS [AMOUNT_IOTX]",
		config.Chinese: "bytecode (合约地址|别名) 已打包参数 [IOTX数量]",
	}
	_testBytecodeCmdShorts = map[config.Language]string{
		config.English: "test smart contract on IoTeX blockchain with packed arguments",
		config.Chinese: "传入bytecode测试IoTeX区块链上的智能合约",
	}
)

// NewContractTestBytecodeCmd represents the contract test bytecode command
func NewContractTestBytecodeCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_testBytecodeCmdUses)
	short, _ := client.SelectTranslation(_testBytecodeCmdShorts)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			addr, err := client.Address(args[0])
			if err != nil {
				return errors.Wrap(err, "failed to get contract address")
			}

			contract, err := address.FromString(addr)
			if err != nil {
				return errors.Wrap(err, "failed to convert string into address")
			}

			bytecode, err := decodeBytecode(args[1])
			if err != nil {
				return errors.Wrap(err, "invalid bytecode")
			}

			amount := big.NewInt(0)
			if len(args) == 3 {
				amount, err = util.StringToRau(args[2], util.IotxDecimalNum)
				if err != nil {
					return errors.Wrap(err, "invalid amount")
				}
			}

			_, signer, _, _, gasLimit, _, err := action.GetWriteCommandFlag(cmd)
			if err != nil {
				return err
			}
			result, err := action.Read(client, contract, amount.String(), bytecode, signer, gasLimit)
			if err != nil {
				return errors.Wrap(err, "failed to read contract")
			}

			cmd.Printf("return: %s\n", result)
			return nil
		},
	}
	action.RegisterWriteCommand(client, cmd)
	return cmd
}
