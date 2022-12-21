// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package did

import (
	"encoding/hex"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/action"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/alias"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	_getHashCmdUses = map[config.Language]string{
		config.English: "gethash (CONTRACT_ADDRESS|ALIAS) DID",
		config.Chinese: "gethash (合约地址|别名) DID",
	}
	_getHashCmdShorts = map[config.Language]string{
		config.English: "Get DID doc's hash on IoTeX blockchain",
		config.Chinese: "在IoTeX链上获取相应DID的doc hash",
	}
)

// NewDidGetHashCmd represents the did get hash command
func NewDidGetHashCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_getHashCmdUses)
	short, _ := client.SelectTranslation(_getHashCmdShorts)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			addr, err := alias.IOAddress(client, args[0])
			if err != nil {
				return errors.Wrap(err, "failed to get contract address")
			}
			bytecode, err := _didABI.Pack(_getHashName, []byte(args[1]))
			if err != nil {
				return errors.Wrap(err, "invalid bytecode")
			}
			result, err := action.Read(client, addr, "0", bytecode, action.SignerFlagDefault, action.GasLimitFlagDefault)
			if err != nil {
				return errors.Wrap(err, "failed to read contract")
			}
			ret, err := hex.DecodeString(result)
			if err != nil {
				return errors.Wrap(err, "failed to decode contract")
			}
			res, err := _didABI.Unpack(_getHashName, ret)
			if err != nil {
				return errors.New("DID does not exist")
			}
			out, err := util.To32Bytes(res[0])
			if err != nil {
				return errors.Wrap(err, "failed to convert hash to bytes")
			}
			cmd.Println(hex.EncodeToString(out[:]))
			return nil
		},
	}
	return cmd
}
