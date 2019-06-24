// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
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
		address, err = config.GetContext()
		if err != nil {
			return "", err
		}
	}
	addr, err := alias.Address(address)
	if err != nil {
		return "", err
	}
	fmt.Printf("Enter password #%s:\n", addr)
	bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.L().Error("failed to get password", zap.Error(err))
		return "", err
	}
	return Sign(addr, string(bytePassword), msg)
}
