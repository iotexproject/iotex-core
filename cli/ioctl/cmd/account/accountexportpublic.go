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

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// accountExportPublicCmd represents the account export public key command
var accountExportPublicCmd = &cobra.Command{
	Use:   "exportpublic (ALIAS|ADDRESS)",
	Short: "Export IoTeX public key from wallet",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := accountExportPublic(args)
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

func accountExportPublic(args []string) (string, error) {
	addr, err := alias.Address(args[0])
	if err != nil {
		return "", err
	}
	fmt.Printf("Enter password #%s:\n", args[0])
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		log.L().Error("failed to get password", zap.Error(err))
		return "", err
	}
	prvKey, err := KsAccountToPrivateKey(addr, password)
	if err != nil {
		return "", err
	}
	defer prvKey.Zero()
	return prvKey.PublicKey().HexString(), nil
}
