// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"fmt"
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	_totalSupplyCmdShorts = map[config.Language]string{
		config.English: "Get total supply",
		config.Chinese: "获得总供应",
	}
	_totalSupplyCmdUses = map[config.Language]string{
		config.English: "totalSupply -c ALIAS|CONTRACT_ADDRESS",
		config.Chinese: "totalSupply -c 别名|合同地址",
	}
)

// _xrc20TotalSupplyCmd represents total supply of the contract
var _xrc20TotalSupplyCmd = &cobra.Command{
	Use:   config.TranslateInLang(_totalSupplyCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_totalSupplyCmdShorts, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := totalSupply()
		return output.PrintError(err)
	},
}

func totalSupply() error {
	bytecode, err := _xrc20ABI.Pack("totalSupply")
	if err != nil {
		return output.NewError(output.ConvertError, "cannot generate bytecode from given command", err)
	}
	contract, err := xrc20Contract()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get contract address", err)
	}
	result, err := Read(contract, "0", bytecode)
	if err != nil {
		return output.NewError(0, "failed to read contract", err)
	}
	decimal, ok := new(big.Int).SetString(result, 16)
	if !ok {
		return errors.New("failed to set contract supply")
	}
	message := amountMessage{RawData: result, Decimal: decimal.String()}
	fmt.Println(message.String())
	return err
}
