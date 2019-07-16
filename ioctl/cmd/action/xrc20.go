// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/ioctl/cmd/config"
)

//Xrc20Cmd represent erc20 standard command-line
var Xrc20Cmd = &cobra.Command{
	Use:   "xrc20",
	Short: "Supporting ERC20 standard command-line from ioctl",
}

var xrc20ContractAddress string

func xrc20Contract() (address.Address, error) {
	return alias.IOAddress(xrc20ContractAddress)
}

func init() {
	Xrc20Cmd.AddCommand(xrc20TotalSupplyCmd)
	Xrc20Cmd.AddCommand(xrc20BalanceOfCmd)
	Xrc20Cmd.AddCommand(xrc20TransferCmd)
	Xrc20Cmd.AddCommand(xrc20TransferFromCmd)
	Xrc20Cmd.AddCommand(xrc20ApproveCmd)
	Xrc20Cmd.AddCommand(xrc20AllowanceCmd)
	Xrc20Cmd.PersistentFlags().StringVarP(&xrc20ContractAddress, "contract-address", "c", "",
		"set contract address")
	Xrc20Cmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, "set endpoint for once")
	Xrc20Cmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure,
		"insecure connection for once (default false)")
	cobra.MarkFlagRequired(Xrc20Cmd.PersistentFlags(), "contract-address")
}

func parseAmount(contract address.Address, amount string) (*big.Int, error) {
	decimalBytecode, err := hex.DecodeString("313ce567")
	if err != nil {
		return nil, err
	}
	output, err := read(contract, decimalBytecode)
	if err != nil {
		return nil, err
	}
	var decimal int64
	if output != "" {
		decimal, err = strconv.ParseInt(output, 16, 8)
		if err != nil {
			return nil, err
		}
	} else {
		decimal = int64(0)
	}
	amountFloat, ok := (*big.Float).SetString(new(big.Float), amount)
	if !ok {
		return nil, err
	}
	amountResultFloat := amountFloat.Mul(amountFloat, new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(decimal), nil)))
	if !amountResultFloat.IsInt() {
		fmt.Println("Please enter appropriate amount")
		return nil, errors.Wrap(err, "Unappropriate amount")
	}
	var amountResultInt *big.Int
	amountResultInt, _ = amountResultFloat.Int(amountResultInt)
	return amountResultInt, nil
}
