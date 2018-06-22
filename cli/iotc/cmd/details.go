// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/logger"
)

// detailsCmd represents the details command
var detailsCmd = &cobra.Command{
	Use:   "details [addr]",
	Short: "Returns the details of given account",
	Long:  `Returns the details of given account, namely the balance and the nonce.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		details(args)
	},
}

func details(args []string) {
	client, _ := getClientAndCfg()
	det, err := client.GetAddressDetails(args[0])
	if err != nil {
		logger.Error().Err(err).Msgf("cannot get details for address %s", args[0])
		return
	}
	fmt.Printf("Address %s nonce: %d\n", args[0], det.Nonce)
	fmt.Printf("Address %s balance: %d\n", args[0], det.TotalBalance)
}

func init() {
	rootCmd.AddCommand(detailsCmd)
}
