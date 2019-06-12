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

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// Xrc20BalanceofCmd represents balanceof function
var Xrc20BalanceofCmd = &cobra.Command{
	Use: "balanceof" +
		" -c ALIAS|CONTRACT_ADDRESS (ALIAS|OWNER_ADDRESS)",
	Short: "Get account balance",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		addr, err := alias.Address(args[0])
		if err != nil {
			return err
		}
		ownerAddress, err = address.FromString(addr)
		if err != nil {
			return err
		}
		output, err := balanceOf(args)
		if err == nil {
			fmt.Println(output)
			result, _ := strconv.ParseUint(output, 16, 64)
			fmt.Println("Output in decimal format:")
			fmt.Println(uint64(result))
		}
		return err
	},
}

func toEthAddr(addr address.Address) common.Address {
	return common.BytesToAddress(addr.Bytes())
}

// read reads smart contract on IoTeX blockchain
func balanceOf(args []string) (string, error) {
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

	bytes, err := abiResult.Pack("balanceOf", toEthAddr(ownerAddress))
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
