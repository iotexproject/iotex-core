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

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

//Xrc20Cmd represent erc20 standard command-line
var Xrc20Cmd = &cobra.Command{
	Use:   "xrc20",
	Short: "Support ERC20 standard command-line from ioctl",
}

var xrc20ContractAddress string

func xrc20Contract() (address.Address, error) {
	addr, err := alias.IOAddress(xrc20ContractAddress)
	if err != nil {
		return nil, output.NewError(output.FlagError, "invalid xrc20 address flag", err)
	}
	return addr, nil
}

type amountMessage struct {
	RawData string `json:"rawData"`
	Decimal string `json:"decimal"`
}

func (m *amountMessage) String() string {
	if output.Format == "" {
		return fmt.Sprintf("Raw output: %s\nOutput in decimal: %s", m.RawData, m.Decimal)
	}
	return output.FormatString(output.Result, m)
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
		return nil, output.NewError(output.ConvertError, "failed to decode 313ce567", err)
	}
	result, err := Read(contract, decimalBytecode)
	if err != nil {
		return nil, output.NewError(0, "failed to read contract", err)
	}
	var decimal int64
	if result != "" {
		decimal, err = strconv.ParseInt(result, 16, 8)
		if err != nil {
			return nil, output.NewError(output.ConvertError, "failed to convert string into int64", err)
		}
	} else {
		decimal = int64(0)
	}
	amountFloat, ok := (*big.Float).SetString(new(big.Float), amount)
	if !ok {
		return nil, output.NewError(output.ConvertError, "failed to convert string into bit float", err)
	}
	amountResultFloat := amountFloat.Mul(amountFloat, new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10),
		big.NewInt(decimal), nil)))
	if !amountResultFloat.IsInt() {
		return nil, output.NewError(output.ValidationError, "unappropriated amount", nil)
	}
	var amountResultInt *big.Int
	amountResultInt, _ = amountResultFloat.Int(amountResultInt)
	return amountResultInt, nil
}
