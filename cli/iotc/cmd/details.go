// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/pkg/log"
)

// detailsCmd represents the details command
var detailsCmd = &cobra.Command{
	Use:   "details [addr]",
	Short: "Returns the details of given account",
	Long:  `Returns the details of given account, namely the balance and the nonce.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(details(args))
	},
}

func details(args []string) string {
	client, err := getClient()
	if err != nil {
		log.L().Error("Cannot get explorer client.", zap.Error(err))
		return ""
	}
	det, err := client.GetAddressDetails(args[0])
	if err != nil {
		log.S().Errorf("Cannot get details for address %s: %v.", args[0], err)
		return ""
	}
	return fmt.Sprintf("Address %s nonce: %d\n", args[0], det.Nonce) +
		fmt.Sprintf("Address %s balance: %s", args[0], det.TotalBalance)
}

func init() {
	rootCmd.AddCommand(detailsCmd)
}
