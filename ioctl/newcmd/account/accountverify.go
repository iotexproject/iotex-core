// Copyright (c) 2022 IoTeX Foundation
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
	_verifyCmdShorts = map[config.Language]string{
		config.English: "Verify IoTeX public key and address by private key",
		config.Chinese: "用私钥验证IoTeX的公钥和地址",
	}
	_enterPrivateKey = map[config.Language]string{
		config.English: "Enter private key:",
		config.Chinese: "输入私钥:",
	}
	_failToGetPrivateKey = map[config.Language]string{
		config.English: "failed to get private key",
		config.Chinese: "获取私钥失败",
	}
	_failToCovertHexStringToPrivateKey = map[config.Language]string{
		config.English: "failed to covert hex string to private key",
		config.Chinese: "十六进制字符串转换私钥失败",
	}
)

// NewAccountVerify represents the account verify command
func NewAccountVerify(client ioctl.Client) *cobra.Command {
	short, _ := client.SelectTranslation(_verifyCmdShorts)
	_enterPrivateKey, _ := client.SelectTranslation(_enterPrivateKey)
	_failToGetPrivateKey, _ := client.SelectTranslation(_failToGetPrivateKey)
	failToConvertPublicKeyIntoAddress, _ := client.SelectTranslation(_failToConvertPublicKeyIntoAddress)
	_failToCovertHexStringToPrivateKey, _ := client.SelectTranslation(_failToCovertHexStringToPrivateKey)

	return &cobra.Command{
		Use:   "verify",
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			cmd.Println(_enterPrivateKey)
			privateKey, err := client.ReadSecret()
			if err != nil {
				return errors.Wrap(err, _failToGetPrivateKey)
			}
			priKey, err := crypto.HexStringToPrivateKey(privateKey)
			if err != nil {
				return errors.Wrap(err, _failToCovertHexStringToPrivateKey)
			}
			addr := priKey.PublicKey().Address()
			if addr == nil {
				return errors.New(failToConvertPublicKeyIntoAddress)
			}
			priKey.Zero()
			cmd.Println(fmt.Sprintf("Address:\t%s\nPublic Key:\t%s",
				addr.String(),
				fmt.Sprintf("%x", priKey.PublicKey().Bytes())),
			)
			return nil
		},
	}
}
