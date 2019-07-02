// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// accountSignCmd represents the account sign command
var accountSignCmd = &cobra.Command{
	Use:   "sign [ALIAS|ADDRESS] MESSAGE",
	Short: "Sign message with private key from wallet",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := accountSign(args)
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

func accountSign(args []string) (string, error) {
	var (
		address string
		msg     string
		err     error
	)
	if len(args) == 2 {
		address = args[0]
		msg = args[1]
	} else {
		msg = args[0]
		address, err = config.GetContextAddressOrAlias()
		if err != nil {
			return "", err
		}
	}
	addr, err := util.Address(address)
	if err != nil {
		return "", err
	}
	fmt.Printf("Enter password #%s:\n", addr)
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		log.L().Error("failed to get password", zap.Error(err))
		return "", err
	}
	return Sign(addr, password, msg)
}
