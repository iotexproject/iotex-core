// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"strings"
	"fmt"

	"github.com/iotexproject/go-ethereum/crypto"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/address"
)

var numAccounts int

// accountcreateCmd represents the accountcreate command
var accountcreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create N new accounts and print them",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(accountCreate(args))
	},
}

func init() {
	accountcreateCmd.Flags().IntVarP(&numAccounts, "num", "n", 1, "number of accounts to create")

	accountCmd.AddCommand(accountcreateCmd)
}

func accountCreate(args []string) string {
	items := make([]string, numAccounts)
	for i := 0; i < numAccounts; i++ {
		private, err := crypto.GenerateKey()
		if err != nil {
			log.L().Fatal("failed to create key pair", zap.Error(err))
		}
		pkHash := keypair.HashPubKey(&private.PublicKey)
		addr, _ := address.FromBytes(pkHash[:])
		pubKeyBytes := keypair.PublicKeyToBytes(&private.PublicKey)
		priKeyBytes := keypair.PrivateKeyToBytes(private)
		items[i] = fmt.Sprintf(
			"{\"Address\": \"%s\", \"PublicKey\": \"%x\", \"PrivateKey\": \"%x\"}",
			addr.String(),
			pubKeyBytes,
			priKeyBytes,
		)
	}
	return "[" + strings.Join(items, ",") + "]"
}