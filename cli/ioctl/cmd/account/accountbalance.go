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

	"github.com/iotexproject/iotex-core/pkg/log"
)

// TODO: use wallet config later
var configAddress = "ioaddress"

// accountBalanceCmd represents the account balance command
var accountBalanceCmd = &cobra.Command{
	Use:   "balance []address",
	Short: "Get balance of an account",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(balance(args))
	},
}

// Balance gets balance of an IoTex blockchain address
func balance(args []string) string {
	lines := make([]string, 0)
	if len(args) == 0 {
		accountMeta, err := GetAccountMeta(configAddress)
		if err != nil {
			log.L().Error("cannot get account from "+configAddress, zap.Error(err))
			return err.Error()
		}
		lines = append(lines, fmt.Sprintf("%s: %s", configAddress, accountMeta.Balance))
	} else {
		for _, addr := range args {
			accountMeta, err := GetAccountMeta(addr)
			if err != nil {
				log.L().Error("cannot get account from "+addr, zap.Error(err))
				return err.Error()
			}
			lines = append(lines, fmt.Sprintf("%s: %s", addr, accountMeta.Balance))
		}
	}

	return strings.Join(lines, "\n")
}
