// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/logger"
)

// balanceCmd represents the balance command
var balanceCmd = &cobra.Command{
	Use:   "balance [addr]",
	Short: "Returns the current balance of given address",
	Long:  `Returns the current balance of given address.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		balance(args)
	},
}

func balance(args []string) {
	client, _ := getClientAndCfg()
	balance, err := client.GetAddressBalance(args[0])
	if err != nil {
		logger.Error().Err(err).Msgf("cannot get balance for address %s", args[0])
		return
	}
	fmt.Printf("Address %s balance: %d\n", args[0], balance)
}

func init() {
	rootCmd.AddCommand(balanceCmd)
}
