// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	verifyCmdShorts = map[config.Language]string{
		config.English: "Verify IoTeX public key and address by private key",
		config.Chinese: "用私钥验证IoTeX的公钥和地址",
	}
	verifyCmdUses = map[config.Language]string{
		config.English: "verify",
		config.Chinese: "verify 验证",
	}
	failToCovertHexStringToPrivateKey = map[config.Language]string{
		config.English: "failed to covert hex string to private key",
		config.Chinese: "十六进制字符串转换私钥失败",
	}
)

// NewAccountVerify represents the account verify command
func NewAccountVerify(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(verifyCmdUses)
	short, _ := client.SelectTranslation(verifyCmdShorts)
	failToConvertPublicKeyIntoAddress, _ := client.SelectTranslation(failToConvertPublicKeyIntoAddress)
	failToCovertHexStringToPrivateKey, _ := client.SelectTranslation(failToCovertHexStringToPrivateKey)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			priKey, err := crypto.HexStringToPrivateKey(args[0])
			if err != nil {
				return errors.Wrap(err, failToCovertHexStringToPrivateKey)
			}
			addr := priKey.PublicKey().Address()
			if addr == nil {
				return errors.New(failToConvertPublicKeyIntoAddress)
			}
			message := verifyMessage{
				Address:   addr.String(),
				PublicKey: fmt.Sprintf("%x", priKey.PublicKey().Bytes()),
			}
			priKey.Zero()
			client.PrintInfo(message.String())
			return nil
		},
	}
	return cmd
}

type verifyMessage struct {
	Address   string `json:"address"`
	PublicKey string `json:"publicKey"`
}

func (m *verifyMessage) String() string {
	return fmt.Sprintf("Address:\t%s\nPublic Key:\t%s", m.Address, m.PublicKey)
}
