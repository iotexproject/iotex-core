// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"fmt"
	"math/big"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// xrc20AllowanceCmd represents your signer limited amount on target address
var xrc20AllowanceCmd = &cobra.Command{
	Use: "allowance [-s SIGNER] (ALIAS|SPENDER_ADDRESS) " +
		" -c ALIAS|CONTRACT_ADDRESS ",
	Short: "the amount which spender is still allowed to withdraw from owner",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		caller, err := signer()
		if err != nil {
			return err
		}
		owner, err := alias.EtherAddress(caller)
		if err != nil {
			return err
		}
		spender, err := alias.EtherAddress(args[0])
		if err != nil {
			return err
		}
		bytecode, err := xrc20ABI.Pack("allowance", owner, spender)
		if err != nil {
			log.L().Error("cannot generate bytecode from given command", zap.Error(err))
			return err
		}
		contract, err := xrc20Contract()
		if err != nil {
			return err
		}
		output, err := read(contract, bytecode)
		if err == nil {
			fmt.Println("Raw output:", output)
			decimal, _ := new(big.Int).SetString(output, 16)
			fmt.Printf("Output in decimal: %d\n", decimal)
		}
		return err
	},
}
