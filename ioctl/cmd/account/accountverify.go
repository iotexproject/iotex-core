// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

var (
	// accountVerifyCmd represents the account verify command
	accountVerifyCmd = &cobra.Command{
		Use:   "verify",
		Short: "Verify IoTeX public key and address by private key",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			err := accountVerify()
			return err
		},
	}
)

type verifyMessage struct {
	Address   string `json:"address"`
	PublicKey string `json:"publicKey"`
}

func accountVerify() error {
	fmt.Println("Enter private key:")
	privateKey, err := util.ReadSecretFromStdin()
	if err != nil {
		return output.PrintError(output.InputError, err.Error())
	}
	priKey, err := crypto.HexStringToPrivateKey(privateKey)
	if err != nil {
		return output.PrintError(output.CryptoError, err.Error())
	}
	addr, err := address.FromBytes(priKey.PublicKey().Hash())
	if err != nil {
		return output.PrintError(output.ConvertError, err.Error())
	}
	message := verifyMessage{
		Address:   addr.String(),
		PublicKey: fmt.Sprintf("%x", priKey.PublicKey().Bytes()),
	}
	priKey.Zero()
	fmt.Println(message.String())
	return nil
}

func (m *verifyMessage) String() string {
	if output.Format == "" {
		return fmt.Sprintf("Address:\t%s\nPublic Key:\t%s", m.Address, m.PublicKey)
	}
	return output.FormatString(output.Result, m)
}
