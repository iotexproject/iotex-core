// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"encoding/hex"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
)

// selfCmd represents the self command
var selfCmd = &cobra.Command{
	Use:   "self",
	Short: "Returns this node's address",
	Long:  `Returns this node's address`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(self())
	},
}

func self() string {
	cfg, err := getCfg()
	if err != nil {
		logger.Error().Err(err).Msg("unable to find config file")
		return ""
	}
	pubk, err := hex.DecodeString(cfg.Chain.ProducerPubKey)
	if err != nil {
		logger.Error().Err(err).Msg("unable to decode pubkey")
		return ""
	}
	addr, err := iotxaddress.GetAddress(pubk, iotxaddress.IsTestnet, iotxaddress.ChainID)
	if err != nil {
		logger.Error().Err(err).Msg("unable to construct address from pubkey")
		return ""
	}

	rawAddr := addr.RawAddress
	return fmt.Sprintf("this node's address is %s", rawAddr)
}

func init() {
	rootCmd.AddCommand(selfCmd)
}
