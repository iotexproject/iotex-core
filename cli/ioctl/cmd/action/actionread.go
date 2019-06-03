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

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// actionReadCmd represents the action read command
var actionReadCmd = &cobra.Command{
	Use: "read (ALIAS|CONTRACT_ADDRESS)" +
		" -s SIGNER -b BYTE_CODE -l GAS_LIMIT [-p GAS_PRICE]",
	Short: "Read smart contract on IoTeX blockchain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := read(args)
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

// read reads smart contract on IoTeX blockchain
func read(args []string) (string, error) {
	contract, err := alias.Address(args[0])
	if err != nil {
		return "", err
	}
	executor, err := alias.Address(signer)
	if err != nil {
		return "", err
	}
	tx, err := inputToExecution(contract, big.NewInt(0))
	if err != nil || tx == nil {
		log.L().Error("cannot make a Execution instance", zap.Error(err))
		return "", err
	}
	return readAction(tx, executor)
}
