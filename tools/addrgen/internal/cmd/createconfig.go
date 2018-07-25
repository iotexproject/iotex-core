// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"io/ioutil"
	"log"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/iotxaddress"
)

// createConfigCmd represents the create-config command
var createConfigCmd = &cobra.Command{
	Use:   "create-config [# output-file]",
	Short: "Creates a yaml config using generated pub/pri key pair.",
	Long:  `Creates a yaml config using generated pub/pri key pair.`,
	Run: func(cmd *cobra.Command, args []string) {
		addr, err := iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)
		if err != nil {
			log.Fatal(err)
		}
		cfgStr := fmt.Sprintf(
			`chain:
  producerPrivKey: "%x"
  producerPubKey: "%x"
`,
			addr.PrivateKey,
			addr.PublicKey,
		)
		if err := ioutil.WriteFile(_outputFile, []byte(cfgStr), 0666); err != nil {
			log.Fatal(err)
		}
	},
}

var _outputFile string

func init() {
	createConfigCmd.Flags().StringVarP(&_outputFile, "output-file", "o", "", "config output file")
	createConfigCmd.MarkFlagRequired("output-file")
	rootCmd.AddCommand(createConfigCmd)
}
