// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"io/ioutil"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// createConfigCmd represents the create-config command
var createConfigCmd = &cobra.Command{
	Use:   "create-config [# output-file]",
	Short: "Creates a yaml config using generated pub/pri key pair.",
	Long:  `Creates a yaml config using generated pub/pri key pair.`,
	Run: func(cmd *cobra.Command, args []string) {
		private, err := crypto.GenerateKey()
		if err != nil {
			log.L().Fatal("failed to create key pair", zap.Error(err))
		}
		priKeyBytes := keypair.PrivateKeyToBytes(private)
		pubKeyBytes := keypair.PublicKeyToBytes(&private.PublicKey)
		cfgStr := fmt.Sprintf(
			`chain:
  producerPrivKey: "%x"
  producerPubKey: "%x"
`,
			priKeyBytes,
			pubKeyBytes,
		)
		if err := ioutil.WriteFile(_outputFile, []byte(cfgStr), 0666); err != nil {
			log.L().Fatal("failed to write file", zap.Error(err))
		}
	},
}

var _outputFile string

func init() {
	createConfigCmd.Flags().StringVarP(&_outputFile, "output-file", "o", "", "config output file")
	createConfigCmd.MarkFlagRequired("output-file")
	rootCmd.AddCommand(createConfigCmd)
}
