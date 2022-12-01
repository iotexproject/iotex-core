// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package did

import (
	"encoding/hex"
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/action"
)

// Multi-language support
var (
	_registerCmdUses = map[config.Language]string{
		config.English: "register (CONTRACT_ADDRESS|ALIAS) hash uri [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "register (合约地址|别名) hash uri [-s 签署人] [-n NONCE] [-l GAS限制] [-P GAS价格] [-P 密码] [-y]",
	}
	_registerCmdShorts = map[config.Language]string{
		config.English: "Register DID on IoTeX blockchain",
		config.Chinese: "在IoTeX链上注册DID",
	}
)

// NewDidRegisterCmd represents the did register command
func NewDidRegisterCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_registerCmdUses)
	short, _ := client.SelectTranslation(_registerCmdShorts)

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
			bytecode, err := encode(_registerDIDName, args[1], args[2])
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

func encode(method, didHash, uri string) ([]byte, error) {
	hashSlice, err := hex.DecodeString(didHash)
	if err != nil {
		return nil, err
	}
	var hashArray [32]byte
	copy(hashArray[:], hashSlice)
	return _didABI.Pack(method, hashArray, []byte(uri))
}
