// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"fmt"
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/output"
)

// xrc20TotalSupplyCmd represents total supply of the contract
var xrc20TotalSupplyCmd = &cobra.Command{
	Use:   "totalSupply -c ALIAS|CONTRACT_ADDRESS",
	Short: "Get total supply",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		bytecode, err := xrc20ABI.Pack("totalSupply")
		if err != nil {
			return output.PrintError(0, "cannot generate bytecode from given command"+err.Error()) // TODO: undefined error
		}
		contract, err := xrc20Contract()
		if err != nil {
			return output.PrintError(output.AddressError, err.Error())
		}
		result, err := read(contract, bytecode)
		if err != nil {
			return output.PrintError(0, err.Error()) // TODO: undefined error
		}
		decimal, _ := new(big.Int).SetString(result, 16)
		message := amountMessage{RawData: result, Decimal: decimal.String()}
		fmt.Println(message.String())
		return err
	},
}
