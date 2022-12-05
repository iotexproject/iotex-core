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

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/alias"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	_xrc20CmdShorts = map[config.Language]string{
		config.English: "Support ERC20 standard command-line",
		config.Chinese: "使ioctl命令行支持ERC20标准",
	}
	_xrc20ContractAddressUsage = map[config.Language]string{
		config.English: "set contract address",
		config.Chinese: "设定合约地址",
	}
)

// NewXrc20Cmd represent xrc20 standard command-line
func NewXrc20Cmd(client ioctl.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use: "xrc20",
	}
	cmd.Short, _ = client.SelectTranslation(_xrc20CmdShorts)

	// add sub commands
	cmd.AddCommand(NewXrc20TransferFromCmd(client))
	// TODO cmd.AddCommand(NewXrc20TotalSupply(client))
	// TODO cmd.AddCommand(NewXrc20BalanceOf(client))
	// TODO cmd.AddCommand(NewXrc20Transfer(client))
	// TODO cmd.AddCommand(NewXrc20Approve(client))
	// TODO cmd.AddCommand(NewXrc20Allowance(client))

	client.SetEndpointWithFlag(cmd.PersistentFlags().StringVar)
	client.SetInsecureWithFlag(cmd.PersistentFlags().BoolVar)

	var ContractAddressFlag string
	contractAddressUsage, _ := client.SelectTranslation(_xrc20ContractAddressUsage)
	cmd.PersistentFlags().StringVarP(&ContractAddressFlag, "contract-address", "c", "", contractAddressUsage)
	if err := cmd.MarkFlagRequired("contract-address"); err != nil {
		fmt.Printf("failed to set required flag: %v\n", err)
	}
	return cmd
}

func xrc20Contract(client ioctl.Client, contractAddr string) (address.Address, error) {
	addr, err := alias.IOAddress(client, contractAddr)
	if err != nil {
		return nil, errors.Wrap(err, "invalid xrc20 address flag")
	}
	return addr, nil
}

func amountMessage(rawData, decimal string) string {
	return fmt.Sprintf("Raw output: %s\nOutput in decimal: %s", rawData, decimal)
}

func parseAmount(client ioctl.Client, cmd *cobra.Command, contract address.Address, amount string) (*big.Int, error) {
	decimalBytecode, err := hex.DecodeString("313ce567")
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode 313ce567")
	}
	signer, err := cmd.Flags().GetString(signerFlagLabel)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get flag signer")
	}
	gasLimit, err := cmd.Flags().GetUint64(gasLimitFlagLabel)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get flag gas-limit")
	}
	result, err := Read(client, contract, "0", decimalBytecode, signer, gasLimit)
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
