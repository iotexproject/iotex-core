// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"encoding/hex"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core-internal/iotxaddress"
)

// selfCmd represents the self command
var selfCmd = &cobra.Command{
	Use:   "self",
	Short: "Returns this node's address",
	Long:  `Returns this node's address`,
	Run: func(cmd *cobra.Command, args []string) {
		self()
	},
}

func self() {
	_, cfg := getClientAndCfg()
	rawAddr := address(cfg.Chain.ProducerPubKey)
	fmt.Printf("this node's address is %s\n", rawAddr)
}

func address(pubkey string) string {
	pubk, err := hex.DecodeString(pubkey)
	if err != nil {
		panic(err)
	}
	addr, err := iotxaddress.GetAddress(pubk, iotxaddress.IsTestnet, iotxaddress.ChainID)
	if err != nil {
		panic(err)
	}
	return addr.RawAddress
}

func init() {
	rootCmd.AddCommand(selfCmd)
}
