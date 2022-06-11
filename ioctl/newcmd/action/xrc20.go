// Copyright (c) 2022 IoTeX Foundation
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

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	_xrc20CmdShorts = map[config.Language]string{
		config.English: "Support ERC20 standard command-line",
		config.Chinese: "使ioctl命令行支持ERC20标准",
	}
	_xrc20CmdUses = map[config.Language]string{
		config.English: "xrc20",
		config.Chinese: "xrc20",
	}
	_flagContractAddressUsages = map[config.Language]string{
		config.English: "set contract address",
		config.Chinese: "设定合约地址",
	}
	_flagXrc20EndPointUsages = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点",
	}
	_flagXrc20InsecureUsages = map[config.Language]string{
		config.English: "insecure connection for once (default false)",
		config.Chinese: "一次不安全的连接（默认为false）",
	}
)

const (
	contractAddrFlagLabel      = "contract-address"
	contractAddrFlagShortLabel = "c"
)

// NewXrc20 represent xrc20 standard command-line
func NewXrc20(client ioctl.Client) *cobra.Command {
	cmd := &cobra.Command{}

	cmd.Use, _ = client.SelectTranslation(_xrc20CmdUses)
	cmd.Short, _ = client.SelectTranslation(_xrc20CmdShorts)

	// add sub commands
	cmd.AddCommand(NewXrc20TransferFrom(client))
	// TODO cmd.AddCommand(NewXrc20TotalSupply(client))
	// TODO cmd.AddCommand(NewXrc20BalanceOf(client))
	// TODO cmd.AddCommand(NewXrc20Transfer(client))
	// TODO cmd.AddCommand(NewXrc20Approve(client))
	// TODO cmd.AddCommand(NewXrc20Allowance(client))

	var (
		contractAddr string
		endpoint     string
		insecure     bool
	)

	// set persistent flags
	cmd.PersistentFlags().StringVarP(&contractAddr, contractAddrFlagLabel, contractAddrFlagShortLabel, "", client.SelectTranslationText(_flagContractAddressUsages))
	cmd.PersistentFlags().StringVar(&endpoint, "endpoint", client.Config().Endpoint, client.SelectTranslationText(_flagXrc20EndPointUsages))
	cmd.PersistentFlags().BoolVar(&insecure, "insecure", !client.Config().SecureConnect, client.SelectTranslationText(_flagXrc20InsecureUsages))
	_ = cmd.MarkFlagRequired("contract-address")

	return cmd
}

func xrc20Contract(cmd *cobra.Command) (address.Address, error) {
	val, err := cmd.Flags().GetString(contractAddrFlagLabel)
	if err != nil {
		return nil, err
	}
	addr, err := alias.IOAddress(val)
	if err != nil {
		return nil, errors.Wrap(err, "invalid xrc20 address flag")
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

func parseAmount(client ioctl.Client, cmd *cobra.Command, contract address.Address, amount string) (*big.Int, error) {
	decimalBytecode, err := hex.DecodeString("313ce567")
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode 313ce567")
	}
	result, err := Read(client, cmd, contract, "0", decimalBytecode)
	if err != nil {
		return nil, errors.New("failed to read contract")
	}

	var decimal int64
	if result != "" {
		decimal, err = strconv.ParseInt(result, 16, 8)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to convert string into int64")
		}
	} else {
		decimal = int64(0)
	}

	return util.StringToRau(amount, int(decimal))
}
