// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"fmt"
	"math/big"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// actionDeployCmd represents the action deploy command
var actionDeployCmd = &cobra.Command{
	Use:   "deploy -s SIGNER -b BYTE_CODE -l GAS_LIMIT [-p GAS_PRICE]",
	Short: "Deploy smart contract on IoTeX blockchain",
	Args:  cobra.MaximumNArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := deploy()
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

// deploy deploys smart contract on IoTeX blockchain
func deploy() (string, error) {
	tx, err := inputToExecution("", big.NewInt(0))
	if err != nil || tx == nil {
		log.L().Error("cannot make a Execution instance", zap.Error(err))
		return "", err
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
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPriceRau).
		SetGasLimit(gasLimit).
		SetAction(tx).Build()
	return sendAction(elp)
}
