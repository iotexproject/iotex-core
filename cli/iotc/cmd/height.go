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

// heightCmd represents the height command
var heightCmd = &cobra.Command{
	Use:   "height",
	Short: "Returns the current height of the blockchain",
	Long:  `Returns the current height of the blockchain.`,
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(height())
	},
}

func height() string {
	client, err := getClient()
	if err != nil {
		log.L().Error("Cannot get explorer client.", zap.Error(err))
	}
	tip, err := client.GetBlockchainHeight()
	if err != nil {
		log.L().Error("Cannot get blockchain height.", zap.Error(err))
		return ""
	}
	return fmt.Sprintf("blockchain height: %d", tip)
}

func init() {
	rootCmd.AddCommand(heightCmd)
}
