// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
)

var numAccounts uint

// accountCreateCmd represents the account create command
var accountCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create N new accounts and print them",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(accountCreate(args))
	},
}

func init() {
	accountCreateCmd.Flags().UintVarP(&numAccounts, "num", "n", 1, "number of accounts to create")
}

func accountCreate(_ []string) string {
	items := make([]string, numAccounts)
	for i := 0; i < int(numAccounts); i++ {
		private, err := keypair.GenerateKey()
		if err != nil {
			log.L().Fatal("failed to create key pair", zap.Error(err))
		}
		addr, err := address.FromBytes(private.PublicKey().Hash())
		if err != nil {
			log.L().Error("failed to convert bytes into address", zap.Error(err))
			return err.Error()
		}
		priKeyBytes := private.Bytes()
		pubKeyBytes := private.PublicKey().Bytes()
		items[i] = fmt.Sprintf(
			"{address: \"%s\", privateKey: \"%x\", publicKey: \"%x\"}",
			addr.String(), priKeyBytes, pubKeyBytes)
	}
	return strings.Join(items, "\n")
}
