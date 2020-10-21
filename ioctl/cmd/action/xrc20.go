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
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	xrc20CmdShorts = map[config.Language]string{
		config.English: "Support ERC20 standard command-line",
		config.Chinese: "使ioctl命令行支持ERC20标准",
	}
	xrc20CmdUses = map[config.Language]string{
		config.English: "xrc20",
		config.Chinese: "xrc20",
	}
	flagContractAddressUsages = map[config.Language]string{
		config.English: "set contract address",
		config.Chinese: "设定合约地址",
	}
	flagXrc20EndPointUsages = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点",
	}
	flagXrc20InsecureUsages = map[config.Language]string{
		config.English: "insecure connection for once (default false)",
		config.Chinese: "一次不安全的连接（默认为false）",
	}
)

//Xrc20Cmd represent erc20 standard command-line
var Xrc20Cmd = &cobra.Command{
	Use:   config.TranslateInLang(xrc20CmdUses, config.UILanguage),
	Short: config.TranslateInLang(xrc20CmdShorts, config.UILanguage),
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
		config.TranslateInLang(flagContractAddressUsages, config.UILanguage))
	Xrc20Cmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, config.TranslateInLang(flagXrc20EndPointUsages, config.UILanguage))
	Xrc20Cmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure,
		config.TranslateInLang(flagXrc20InsecureUsages, config.UILanguage))
	cobra.MarkFlagRequired(Xrc20Cmd.PersistentFlags(), "contract-address")
}

func parseAmount(contract address.Address, amount string) (*big.Int, error) {
	decimalBytecode, err := hex.DecodeString("313ce567")
	if err != nil {
		return nil, output.NewError(output.ConvertError, "failed to decode 313ce567", err)
	}
	result, err := Read(contract, big.NewInt(0), decimalBytecode)
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

	return util.StringToRau(amount, int(decimal))
}
