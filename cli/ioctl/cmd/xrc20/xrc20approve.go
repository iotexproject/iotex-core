// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package xrc20

import (
	"fmt"
	"math/big"
	"strconv"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// Xrc20ApproveCmd could config target address limited amount
var Xrc20ApproveCmd = &cobra.Command{
	Use: "approve" +
		" -c ALIAS|CONTRACT_ADDRESS -s SIGNER -l GAS_LIMIT (ALIAS|SPENDER_ADDRESS) (ALIAS|AMOUNT)",
	Short: "Allow spender to withdraw from your account, multiple times, up to the amount",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		addr, err := alias.Address(args[0])
		if err != nil {
			return err
		}
		spenderAddress, err = address.FromString(addr)
		if err != nil {
			return err
		}
		transfer, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return err
		}
		transferAmount = uint64(transfer)
		output, err := approve(args)
		if err == nil {
			fmt.Println(output)

		}
		return err
	},
}

// read reads smart contract on IoTeX blockchain
func approve(args []string) (string, error) {
	contract, err := alias.Address(contractAddress)
	if err != nil {
		return "", err
	}
	executor, err := alias.Address(signer)
	if err != nil {
		return "", err
	}
	amount := big.NewInt(0)
	if nonce == 0 {
		accountMeta, err := account.GetAccountMeta(executor)
		if err != nil {
			return "", err
		}
		nonce = accountMeta.PendingNonce
	}
	var gasPriceRau *big.Int
	if len(gasPrice) == 0 {
		gasPriceRau, err = GetGasPrice()
		if err != nil {
			return "", err
		}
	} else {
		gasPriceRau, err = util.StringToRau(gasPrice, util.GasPriceDecimalNum)
		if err != nil {
			return "", err
		}
	}

	bytes, err := abiResult.Pack("approve", toEthAddr(spenderAddress), new(big.Int).SetUint64(transferAmount))
	if err != nil {
		return "", err
	}
	tx, err := action.NewExecution(contract, nonce, amount, gasLimit, gasPriceRau, bytes)
	if err != nil || tx == nil {
		log.L().Error("cannot make a Execution instance", zap.Error(err))
		return "", err
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPriceRau).
		SetGasLimit(gasLimit).
		SetAction(tx).Build()
	return sendAction(elp)
}
