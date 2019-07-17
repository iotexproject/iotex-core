// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// accountSignCmd represents the account sign command
var accountSignCmd = &cobra.Command{
	Use:   "sign [ALIAS|ADDRESS] MESSAGE",
	Short: "Sign message with private key from wallet",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := accountSign(args)
		return err
	},
}

func accountSign(args []string) error {
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
			return output.PrintError(output.ConfigError, err.Error())
		}
	}
	addr, err := util.Address(address)
	if err != nil {
		return output.PrintError(output.AddressError, err.Error())
	}
	fmt.Printf("Enter password #%s:\n", addr)
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return output.PrintError(output.InputError, "failed to get password")
	}
	signedMessage, err := Sign(addr, password, msg)
	if err != nil {
		return output.PrintError(output.KeystoreError, err.Error())
	}
	output.PrintResult(signedMessage)
	return nil
}
