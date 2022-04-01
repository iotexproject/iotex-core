// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package did

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
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

// _didDeregisterCmd represents the contract invoke deregister command
var _didDeregisterCmd = &cobra.Command{
	Use:   config.TranslateInLang(_deregisterCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_deregisterCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := deregisterDID(args)
		return output.PrintError(err)
	},
}

func init() {
	action.RegisterWriteCommand(_didDeregisterCmd)
}

func deregisterDID(args []string) (err error) {
	contract, err := util.Address(args[0])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get contract address", err)
	}

	abi, err := abi.JSON(strings.NewReader(DIDABI))
	if err != nil {
		return
	}
	_, exist := abi.Methods[_deregisterDIDName]
	if !exist {
		return errors.New("method is not found")
	}
	bytecode, err := abi.Pack(_deregisterDIDName)
	if err != nil {
		return
	}
	return action.Execute(contract, big.NewInt(0), bytecode)
}
