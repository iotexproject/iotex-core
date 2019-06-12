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

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// Xrc20TotalsupplyCmd represents total supply of the contract
var Xrc20TotalsupplyCmd = &cobra.Command{
	Use: "totalsupply" +
		" -c CONTRACT_ADDRESS",
	Short: "Get total supply",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := totalSupply(args)
		if err == nil {
			fmt.Println(output)
			result, _ := strconv.ParseUint(output, 16, 64)
			fmt.Println("Output in decimal format:")
			fmt.Println(uint64(result))
		}
		return err
	},
}

// read reads smart contract on IoTeX blockchain
func totalSupply(args []string) (string, error) {
	contract, err := alias.Address(contractAddress)
	if err != nil {
		return "", err
	}
	executor, err := alias.Address("ALIAS")
	if err != nil {
		return "", err
	}
	if nonce == 0 {
		accountMeta, err := account.GetAccountMeta(executor)
		if err != nil {
			return "", err
		}
		nonce = accountMeta.PendingNonce
	}
	gasLimit = 50000
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

	bytes, err := abiResult.Pack("totalSupply")
	if err != nil {
		return "", err
	}
	tx, err := action.NewExecution(contract, nonce, big.NewInt(0), gasLimit, gasPriceRau, bytes)
	if err != nil || tx == nil {
		log.L().Error("cannot make a Execution instance", zap.Error(err))
		return "", err
	}
	return readAction(tx, executor)
}
