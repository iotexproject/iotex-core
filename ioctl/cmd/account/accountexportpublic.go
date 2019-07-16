// Copyright (c) 2019 IoTeX
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

// accountExportPublicCmd represents the account export public key command
var accountExportPublicCmd = &cobra.Command{
	Use:   "exportpublic [ALIAS|ADDRESS]",
	Short: "Export IoTeX public key from wallet",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := accountExportPublic(args)
		return err
	},
}

func accountExportPublic(args []string) error {
	addr, err := util.GetAddress(args)
	if err != nil {
		return output.PrintError(output.AddressError, err.Error())
	}
	fmt.Printf("Enter password #%s:\n", addr)
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return output.PrintError(output.InputError, "failed to get password")
	}
	prvKey, err := KsAccountToPrivateKey(addr, password)
	if err != nil {
		return output.PrintError(output.KeystoreError, err.Error())
	}
	defer prvKey.Zero()
	output.PrintResult(prvKey.PublicKey().HexString())
	return nil
}
