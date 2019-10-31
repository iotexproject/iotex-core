// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

var signer string

// accountSignCmd represents the account sign command
var accountSignCmd = &cobra.Command{
	Use:   "sign MESSAGE [-s SIGNER]",
	Short: "Sign message with private key from wallet",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := accountSign(args[0])
		return output.PrintError(err)
	},
}

func init() {
	accountSignCmd.Flags().StringVarP(&signer, "signer", "s", "", "choose a signing account")
}

func accountSign(msg string) error {
	addr, err := util.GetAddress(signer)
	if err != nil {
		return output.NewError(output.InputError, "failed to get signer addr", err)
	}
	fmt.Printf("Enter password #%s:\n", addr)
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return output.NewError(output.InputError, "failed to get password", err)
	}
	signedMessage, err := Sign(addr, password, msg)
	if err != nil {
		return output.NewError(output.KeystoreError, "failed to sign message", err)
	}
	output.PrintResult(signedMessage)
	return nil
}
