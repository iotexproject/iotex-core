// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package did

import (
	"encoding/hex"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/action"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	_getURICmdUses = map[config.Language]string{
		config.English: "geturi (CONTRACT_ADDRESS|ALIAS) DID",
		config.Chinese: "geturi (合约地址|别名) DID",
	}
	_getURICmdShorts = map[config.Language]string{
		config.English: "Geturi get DID URI on IoTeX blockchain",
		config.Chinese: "Geturi 在IoTeX链上获取相应DID的uri",
	}
)

// NewDidGetURICmd represents the did get uri command
func NewDidGetURICmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_getURICmdUses)
	short, _ := client.SelectTranslation(_getURICmdShorts)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			contract, err := client.Address(args[0])
			if err != nil {
				return errors.Wrap(err, "failed to get contract address")
			}
			addr, err := address.FromString(contract)
			if err != nil {
				return errors.Wrap(err, "invalid contract address")
			}

			abi, err := abi.JSON(strings.NewReader(DIDABI))
			if err != nil {
				return errors.Wrap(err, "failed to read abi")
			}
			bytecode, err := abi.Pack(_getURIName, []byte(args[1]))
			if err != nil {
				return errors.Wrap(err, "invalid bytecode")
			}

			result, err := action.Read(client, addr, "0", bytecode, contract, 20000000)
			if err != nil {
				return errors.Wrap(err, "failed to read contract")
			}
			ret, err := hex.DecodeString(result)
			if err != nil {
				return errors.Wrap(err, "failed to decode contract")
			}
			res, err := abi.Unpack(_getURIName, ret)
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
