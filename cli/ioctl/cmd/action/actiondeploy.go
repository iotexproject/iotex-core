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
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// actionDeployCmd deploys smart contract on IoTeX blockchain
var actionDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy smart contract on IoTeX blockchain",
	Args:  cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(deploy())
	},
}

// deploy deploys smart contract on IoTeX blockchain
func deploy() string {
	executor, err := account.AliasToAddress(alias)
	if err != nil {
		return err.Error()
	}
	accountMeta, err := account.GetAccountMeta(executor)
	if err != nil {
		return err.Error()
	}
	tx, err := action.NewExecution("", accountMeta.PendingNonce, big.NewInt(0),
		gasLimit, big.NewInt(gasPrice), bytecode)
	if err != nil {
		log.L().Error("cannot make a Execution instance", zap.Error(err))
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(accountMeta.PendingNonce).
		SetGasPrice(big.NewInt(gasPrice)).
		SetGasLimit(gasLimit).
		SetAction(tx).Build()
	return sendAction(elp)
}
