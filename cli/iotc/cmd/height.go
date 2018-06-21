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

// heightCmd represents the height command
var heightCmd = &cobra.Command{
	Use:   "height",
	Short: "Returns the current height of the blockchain",
	Long:  `Returns the current height of the blockchain.`,
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		height()
	},
}

func height() {
	client, _ := getClientAndCfg()
	tip, err := client.GetBlockchainHeight()
	if err != nil {
		logger.Error().Err(err).Msg("cannot get blockchain height")
	}
	fmt.Printf("blockchain height: %d\n", tip)
}

func init() {
	rootCmd.AddCommand(heightCmd)
}
