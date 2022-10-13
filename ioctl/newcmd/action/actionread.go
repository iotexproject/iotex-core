// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/alias"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	_readCmdShorts = map[config.Language]string{
		config.English: "read smart contract on IoTeX blockchain",
		config.Chinese: "读取IoTeX区块链上的智能合约",
	}
	_readCmdUses = map[config.Language]string{
		config.English: "read (ALIAS|CONTRACT_ADDRESS) -b BYTE_CODE [-s SIGNER]",
		config.Chinese: "reads (别名|联系人地址) -b 类型码 [-s 签署人]",
	}
)

// NewActionReadCmd represents the action Read command
func NewActionReadCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_readCmdUses)
	short, _ := client.SelectTranslation(_readCmdShorts)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			contract, err := alias.IOAddress(client, args[0])
			if err != nil {
				return errors.Wrap(err, "failed to get contract address")
			}
			bytecodeFlag, err := cmd.Flags().GetString(bytecodeFlagLabel)
			if err != nil {
				return errors.Wrap(err, "failed to get flag bytecode")
			}
			fmt.Println(bytecodeFlag)
			bytecode, err := hex.DecodeString(util.TrimHexPrefix(bytecodeFlag))
			if err != nil {
				return errors.Wrap(err, "invalid bytecode")
			}
			_, signer, _, _, gasLimit, _, err := GetWriteCommandFlag(cmd)
			if err != nil {
				return err
			}
			result, err := Read(client, contract, "0", bytecode, signer, gasLimit)
			if err != nil {
				return errors.Wrap(err, "failed to Read contract")
			}
			cmd.Println(result)
			return err
		},
	}
	registerBytecodeFlag(client, cmd)
	RegisterWriteCommand(client, cmd)
	return cmd
}
