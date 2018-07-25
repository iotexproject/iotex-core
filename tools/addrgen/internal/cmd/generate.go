// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/iotxaddress"
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate [# number]",
	Short: "Generates n number of iotex address key pairs.",
	Long:  `Generates n number of iotex address key pairs.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Printf("\n%s\n", generate(args))
	},
}

var _addrNum int

func generate(args []string) string {
	var out string
	for i := 0; i < _addrNum; i++ {
		addr, err := iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)
		if err != nil {
			log.Fatal(err)
		}
		out += fmt.Sprintf("Public Key: %x\nPrivate Key: %x\nRaw Address: %s\n\n",
			addr.PublicKey, addr.PrivateKey, addr.RawAddress)
	}
	return out
}

func init() {
	generateCmd.Flags().IntVarP(&_addrNum, "number", "n", 10, "number of addresses to be generated")
	rootCmd.AddCommand(generateCmd)
}
